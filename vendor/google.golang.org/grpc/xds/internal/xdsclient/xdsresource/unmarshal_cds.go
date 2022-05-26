/*
 *
 * Copyright 2021 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package xdsresource

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	v3clusterpb "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	v3corepb "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	v3aggregateclusterpb "github.com/envoyproxy/go-control-plane/envoy/extensions/clusters/aggregate/v3"
	v3tlspb "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/internal/envconfig"
	"google.golang.org/grpc/internal/grpclog"
	"google.golang.org/grpc/internal/pretty"
	"google.golang.org/grpc/internal/xds/matcher"
	"google.golang.org/grpc/xds/internal/xdsclient/xdsresource/version"
	"google.golang.org/protobuf/types/known/anypb"
)

// TransportSocket proto message has a `name` field which is expected to be set
// to this value by the management server.
const transportSocketName = "envoy.transport_sockets.tls"

// UnmarshalCluster processes resources received in an CDS response, validates
// them, and transforms them into a native struct which contains only fields we
// are interested in.
func UnmarshalCluster(opts *UnmarshalOptions) (map[string]ClusterUpdateErrTuple, UpdateMetadata, error) {
	update := make(map[string]ClusterUpdateErrTuple)
	md, err := processAllResources(opts, update)
	return update, md, err
}

func unmarshalClusterResource(r *anypb.Any, f UpdateValidatorFunc, logger *grpclog.PrefixLogger) (string, ClusterUpdate, error) {
	if !IsClusterResource(r.GetTypeUrl()) {
		return "", ClusterUpdate{}, fmt.Errorf("unexpected resource type: %q ", r.GetTypeUrl())
	}

	cluster := &v3clusterpb.Cluster{}
	if err := proto.Unmarshal(r.GetValue(), cluster); err != nil {
		return "", ClusterUpdate{}, fmt.Errorf("failed to unmarshal resource: %v", err)
	}
	logger.Infof("Resource with name: %v, type: %T, contains: %v", cluster.GetName(), cluster, pretty.ToJSON(cluster))
	cu, err := validateClusterAndConstructClusterUpdate(cluster)
	if err != nil {
		return cluster.GetName(), ClusterUpdate{}, err
	}
	cu.Raw = r
	if f != nil {
		if err := f(cu); err != nil {
			return "", ClusterUpdate{}, err
		}
	}

	return cluster.GetName(), cu, nil
}

const (
	defaultRingHashMinSize = 1024
	defaultRingHashMaxSize = 8 * 1024 * 1024 // 8M
	ringHashSizeUpperBound = 8 * 1024 * 1024 // 8M
)

func validateClusterAndConstructClusterUpdate(cluster *v3clusterpb.Cluster) (ClusterUpdate, error) {
	var lbPolicy *ClusterLBPolicyRingHash
	switch cluster.GetLbPolicy() {
	case v3clusterpb.Cluster_ROUND_ROBIN:
		lbPolicy = nil // The default is round_robin, and there's no config to set.
	case v3clusterpb.Cluster_RING_HASH:
		if !envconfig.XDSRingHash {
			return ClusterUpdate{}, fmt.Errorf("unexpected lbPolicy %v in response: %+v", cluster.GetLbPolicy(), cluster)
		}
		rhc := cluster.GetRingHashLbConfig()
		if rhc.GetHashFunction() != v3clusterpb.Cluster_RingHashLbConfig_XX_HASH {
			return ClusterUpdate{}, fmt.Errorf("unsupported ring_hash hash function %v in response: %+v", rhc.GetHashFunction(), cluster)
		}
		// Minimum defaults to 1024 entries, and limited to 8M entries Maximum
		// defaults to 8M entries, and limited to 8M entries
		var minSize, maxSize uint64 = defaultRingHashMinSize, defaultRingHashMaxSize
		if min := rhc.GetMinimumRingSize(); min != nil {
			if min.GetValue() > ringHashSizeUpperBound {
				return ClusterUpdate{}, fmt.Errorf("unexpected ring_hash mininum ring size %v in response: %+v", min.GetValue(), cluster)
			}
			minSize = min.GetValue()
		}
		if max := rhc.GetMaximumRingSize(); max != nil {
			if max.GetValue() > ringHashSizeUpperBound {
				return ClusterUpdate{}, fmt.Errorf("unexpected ring_hash maxinum ring size %v in response: %+v", max.GetValue(), cluster)
			}
			maxSize = max.GetValue()
		}
		if minSize > maxSize {
			return ClusterUpdate{}, fmt.Errorf("ring_hash config min size %v is greater than max %v", minSize, maxSize)
		}
		lbPolicy = &ClusterLBPolicyRingHash{MinimumRingSize: minSize, MaximumRingSize: maxSize}
	default:
		return ClusterUpdate{}, fmt.Errorf("unexpected lbPolicy %v in response: %+v", cluster.GetLbPolicy(), cluster)
	}

	// Process security configuration received from the control plane iff the
	// corresponding environment variable is set.
	var sc *SecurityConfig
	if envconfig.XDSClientSideSecurity {
		var err error
		if sc, err = securityConfigFromCluster(cluster); err != nil {
			return ClusterUpdate{}, err
		}
	}

	ret := ClusterUpdate{
		ClusterName: cluster.GetName(),
		EnableLRS:   cluster.GetLrsServer().GetSelf() != nil,
		SecurityCfg: sc,
		MaxRequests: circuitBreakersFromCluster(cluster),
		LBPolicy:    lbPolicy,
	}

	// Validate and set cluster type from the response.
	switch {
	case cluster.GetType() == v3clusterpb.Cluster_EDS:
		if cluster.GetEdsClusterConfig().GetEdsConfig().GetAds() == nil {
			return ClusterUpdate{}, fmt.Errorf("unexpected edsConfig in response: %+v", cluster)
		}
		ret.ClusterType = ClusterTypeEDS
		ret.EDSServiceName = cluster.GetEdsClusterConfig().GetServiceName()
		return ret, nil
	case cluster.GetType() == v3clusterpb.Cluster_LOGICAL_DNS:
		if !envconfig.XDSAggregateAndDNS {
			return ClusterUpdate{}, fmt.Errorf("unsupported cluster type (%v, %v) in response: %+v", cluster.GetType(), cluster.GetClusterType(), cluster)
		}
		ret.ClusterType = ClusterTypeLogicalDNS
		dnsHN, err := dnsHostNameFromCluster(cluster)
		if err != nil {
			return ClusterUpdate{}, err
		}
		ret.DNSHostName = dnsHN
		return ret, nil
	case cluster.GetClusterType() != nil && cluster.GetClusterType().Name == "envoy.clusters.aggregate":
		if !envconfig.XDSAggregateAndDNS {
			return ClusterUpdate{}, fmt.Errorf("unsupported cluster type (%v, %v) in response: %+v", cluster.GetType(), cluster.GetClusterType(), cluster)
		}
		clusters := &v3aggregateclusterpb.ClusterConfig{}
		if err := proto.Unmarshal(cluster.GetClusterType().GetTypedConfig().GetValue(), clusters); err != nil {
			return ClusterUpdate{}, fmt.Errorf("failed to unmarshal resource: %v", err)
		}
		ret.ClusterType = ClusterTypeAggregate
		ret.PrioritizedClusterNames = clusters.Clusters
		return ret, nil
	default:
		return ClusterUpdate{}, fmt.Errorf("unsupported cluster type (%v, %v) in response: %+v", cluster.GetType(), cluster.GetClusterType(), cluster)
	}
}

// dnsHostNameFromCluster extracts the DNS host name from the cluster's load
// assignment.
//
// There should be exactly one locality, with one endpoint, whose address
// contains the address and port.
func dnsHostNameFromCluster(cluster *v3clusterpb.Cluster) (string, error) {
	loadAssignment := cluster.GetLoadAssignment()
	if loadAssignment == nil {
		return "", fmt.Errorf("load_assignment not present for LOGICAL_DNS cluster")
	}
	if len(loadAssignment.GetEndpoints()) != 1 {
		return "", fmt.Errorf("load_assignment for LOGICAL_DNS cluster must have exactly one locality, got: %+v", loadAssignment)
	}
	endpoints := loadAssignment.GetEndpoints()[0].GetLbEndpoints()
	if len(endpoints) != 1 {
		return "", fmt.Errorf("locality for LOGICAL_DNS cluster must have exactly one endpoint, got: %+v", endpoints)
	}
	endpoint := endpoints[0].GetEndpoint()
	if endpoint == nil {
		return "", fmt.Errorf("endpoint for LOGICAL_DNS cluster not set")
	}
	socketAddr := endpoint.GetAddress().GetSocketAddress()
	if socketAddr == nil {
		return "", fmt.Errorf("socket address for endpoint for LOGICAL_DNS cluster not set")
	}
	if socketAddr.GetResolverName() != "" {
		return "", fmt.Errorf("socket address for endpoint for LOGICAL_DNS cluster not set has unexpected custom resolver name: %v", socketAddr.GetResolverName())
	}
	host := socketAddr.GetAddress()
	if host == "" {
		return "", fmt.Errorf("host for endpoint for LOGICAL_DNS cluster not set")
	}
	port := socketAddr.GetPortValue()
	if port == 0 {
		return "", fmt.Errorf("port for endpoint for LOGICAL_DNS cluster not set")
	}
	return net.JoinHostPort(host, strconv.Itoa(int(port))), nil
}

// securityConfigFromCluster extracts the relevant security configuration from
// the received Cluster resource.
func securityConfigFromCluster(cluster *v3clusterpb.Cluster) (*SecurityConfig, error) {
	if tsm := cluster.GetTransportSocketMatches(); len(tsm) != 0 {
		return nil, fmt.Errorf("unsupport transport_socket_matches field is non-empty: %+v", tsm)
	}
	// The Cluster resource contains a `transport_socket` field, which contains
	// a oneof `typed_config` field of type `protobuf.Any`. The any proto
	// contains a marshaled representation of an `UpstreamTlsContext` message.
	ts := cluster.GetTransportSocket()
	if ts == nil {
		return nil, nil
	}
	if name := ts.GetName(); name != transportSocketName {
		return nil, fmt.Errorf("transport_socket field has unexpected name: %s", name)
	}
	any := ts.GetTypedConfig()
	if any == nil || any.TypeUrl != version.V3UpstreamTLSContextURL {
		return nil, fmt.Errorf("transport_socket field has unexpected typeURL: %s", any.TypeUrl)
	}
	upstreamCtx := &v3tlspb.UpstreamTlsContext{}
	if err := proto.Unmarshal(any.GetValue(), upstreamCtx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal UpstreamTlsContext in CDS response: %v", err)
	}
	// The following fields from `UpstreamTlsContext` are ignored:
	// - sni
	// - allow_renegotiation
	// - max_session_keys
	if upstreamCtx.GetCommonTlsContext() == nil {
		return nil, errors.New("UpstreamTlsContext in CDS response does not contain a CommonTlsContext")
	}

	return securityConfigFromCommonTLSContext(upstreamCtx.GetCommonTlsContext(), false)
}

// common is expected to be not nil.
// The `alpn_protocols` field is ignored.
func securityConfigFromCommonTLSContext(common *v3tlspb.CommonTlsContext, server bool) (*SecurityConfig, error) {
	if common.GetTlsParams() != nil {
		return nil, fmt.Errorf("unsupported tls_params field in CommonTlsContext message: %+v", common)
	}
	if common.GetCustomHandshaker() != nil {
		return nil, fmt.Errorf("unsupported custom_handshaker field in CommonTlsContext message: %+v", common)
	}

	// For now, if we can't get a valid security config from the new fields, we
	// fallback to the old deprecated fields.
	// TODO: Drop support for deprecated fields. NACK if err != nil here.
	sc, _ := securityConfigFromCommonTLSContextUsingNewFields(common, server)
	if sc == nil || sc.Equal(&SecurityConfig{}) {
		var err error
		sc, err = securityConfigFromCommonTLSContextWithDeprecatedFields(common, server)
		if err != nil {
			return nil, err
		}
	}
	if sc != nil {
		// sc == nil is a valid case where the control plane has not sent us any
		// security configuration. xDS creds will use fallback creds.
		if server {
			if sc.IdentityInstanceName == "" {
				return nil, errors.New("security configuration on the server-side does not contain identity certificate provider instance name")
			}
		} else {
			if sc.RootInstanceName == "" {
				return nil, errors.New("security configuration on the client-side does not contain root certificate provider instance name")
			}
		}
	}
	return sc, nil
}

func securityConfigFromCommonTLSContextWithDeprecatedFields(common *v3tlspb.CommonTlsContext, server bool) (*SecurityConfig, error) {
	// The `CommonTlsContext` contains a
	// `tls_certificate_certificate_provider_instance` field of type
	// `CertificateProviderInstance`, which contains the provider instance name
	// and the certificate name to fetch identity certs.
	sc := &SecurityConfig{}
	if identity := common.GetTlsCertificateCertificateProviderInstance(); identity != nil {
		sc.IdentityInstanceName = identity.GetInstanceName()
		sc.IdentityCertName = identity.GetCertificateName()
	}

	// The `CommonTlsContext` contains a `validation_context_type` field which
	// is a oneof. We can get the values that we are interested in from two of
	// those possible values:
	//  - combined validation context:
	//    - contains a default validation context which holds the list of
	//      matchers for accepted SANs.
	//    - contains certificate provider instance configuration
	//  - certificate provider instance configuration
	//    - in this case, we do not get a list of accepted SANs.
	switch t := common.GetValidationContextType().(type) {
	case *v3tlspb.CommonTlsContext_CombinedValidationContext:
		combined := common.GetCombinedValidationContext()
		var matchers []matcher.StringMatcher
		if def := combined.GetDefaultValidationContext(); def != nil {
			for _, m := range def.GetMatchSubjectAltNames() {
				matcher, err := matcher.StringMatcherFromProto(m)
				if err != nil {
					return nil, err
				}
				matchers = append(matchers, matcher)
			}
		}
		if server && len(matchers) != 0 {
			return nil, fmt.Errorf("match_subject_alt_names field in validation context is not supported on the server: %v", common)
		}
		sc.SubjectAltNameMatchers = matchers
		if pi := combined.GetValidationContextCertificateProviderInstance(); pi != nil {
			sc.RootInstanceName = pi.GetInstanceName()
			sc.RootCertName = pi.GetCertificateName()
		}
	case *v3tlspb.CommonTlsContext_ValidationContextCertificateProviderInstance:
		pi := common.GetValidationContextCertificateProviderInstance()
		sc.RootInstanceName = pi.GetInstanceName()
		sc.RootCertName = pi.GetCertificateName()
	case nil:
		// It is valid for the validation context to be nil on the server side.
	default:
		return nil, fmt.Errorf("validation context contains unexpected type: %T", t)
	}
	return sc, nil
}

// gRFC A29 https://github.com/grpc/proposal/blob/master/A29-xds-tls-security.md
// specifies the new way to fetch security configuration and says the following:
//
// Although there are various ways to obtain certificates as per this proto
// (which are supported by Envoy), gRPC supports only one of them and that is
// the `CertificateProviderPluginInstance` proto.
//
// This helper function attempts to fetch security configuration from the
// `CertificateProviderPluginInstance` message, given a CommonTlsContext.
func securityConfigFromCommonTLSContextUsingNewFields(common *v3tlspb.CommonTlsContext, server bool) (*SecurityConfig, error) {
	// The `tls_certificate_provider_instance` field of type
	// `CertificateProviderPluginInstance` is used to fetch the identity
	// certificate provider.
	sc := &SecurityConfig{}
	identity := common.GetTlsCertificateProviderInstance()
	if identity == nil && len(common.GetTlsCertificates()) != 0 {
		return nil, fmt.Errorf("expected field tls_certificate_provider_instance is not set, while unsupported field tls_certificates is set in CommonTlsContext message: %+v", common)
	}
	if identity == nil && common.GetTlsCertificateSdsSecretConfigs() != nil {
		return nil, fmt.Errorf("expected field tls_certificate_provider_instance is not set, while unsupported field tls_certificate_sds_secret_configs is set in CommonTlsContext message: %+v", common)
	}
	sc.IdentityInstanceName = identity.GetInstanceName()
	sc.IdentityCertName = identity.GetCertificateName()

	// The `CommonTlsContext` contains a oneof field `validation_context_type`,
	// which contains the `CertificateValidationContext` message in one of the
	// following ways:
	//  - `validation_context` field
	//    - this is directly of type `CertificateValidationContext`
	//  - `combined_validation_context` field
	//    - this is of type `CombinedCertificateValidationContext` and contains
	//      a `default validation context` field of type
	//      `CertificateValidationContext`
	//
	// The `CertificateValidationContext` message has the following fields that
	// we are interested in:
	//  - `ca_certificate_provider_instance`
	//    - this is of type `CertificateProviderPluginInstance`
	//  - `match_subject_alt_names`
	//    - this is a list of string matchers
	//
	// The `CertificateProviderPluginInstance` message contains two fields
	//  - instance_name
	//    - this is the certificate provider instance name to be looked up in
	//      the bootstrap configuration
	//  - certificate_name
	//    -  this is an opaque name passed to the certificate provider
	var validationCtx *v3tlspb.CertificateValidationContext
	switch typ := common.GetValidationContextType().(type) {
	case *v3tlspb.CommonTlsContext_ValidationContext:
		validationCtx = common.GetValidationContext()
	case *v3tlspb.CommonTlsContext_CombinedValidationContext:
		validationCtx = common.GetCombinedValidationContext().GetDefaultValidationContext()
	case nil:
		// It is valid for the validation context to be nil on the server side.
		return sc, nil
	default:
		return nil, fmt.Errorf("validation context contains unexpected type: %T", typ)
	}
	// If we get here, it means that the `CertificateValidationContext` message
	// was found through one of the supported ways. It is an error if the
	// validation context is specified, but it does not contain the
	// ca_certificate_provider_instance field which contains information about
	// the certificate provider to be used for the root certificates.
	if validationCtx.GetCaCertificateProviderInstance() == nil {
		return nil, fmt.Errorf("expected field ca_certificate_provider_instance is missing in CommonTlsContext message: %+v", common)
	}
	// The following fields are ignored:
	// - trusted_ca
	// - watched_directory
	// - allow_expired_certificate
	// - trust_chain_verification
	switch {
	case len(validationCtx.GetVerifyCertificateSpki()) != 0:
		return nil, fmt.Errorf("unsupported verify_certificate_spki field in CommonTlsContext message: %+v", common)
	case len(validationCtx.GetVerifyCertificateHash()) != 0:
		return nil, fmt.Errorf("unsupported verify_certificate_hash field in CommonTlsContext message: %+v", common)
	case validationCtx.GetRequireSignedCertificateTimestamp().GetValue():
		return nil, fmt.Errorf("unsupported require_sugned_ceritificate_timestamp field in CommonTlsContext message: %+v", common)
	case validationCtx.GetCrl() != nil:
		return nil, fmt.Errorf("unsupported crl field in CommonTlsContext message: %+v", common)
	case validationCtx.GetCustomValidatorConfig() != nil:
		return nil, fmt.Errorf("unsupported custom_validator_config field in CommonTlsContext message: %+v", common)
	}

	if rootProvider := validationCtx.GetCaCertificateProviderInstance(); rootProvider != nil {
		sc.RootInstanceName = rootProvider.GetInstanceName()
		sc.RootCertName = rootProvider.GetCertificateName()
	}
	var matchers []matcher.StringMatcher
	for _, m := range validationCtx.GetMatchSubjectAltNames() {
		matcher, err := matcher.StringMatcherFromProto(m)
		if err != nil {
			return nil, err
		}
		matchers = append(matchers, matcher)
	}
	if server && len(matchers) != 0 {
		return nil, fmt.Errorf("match_subject_alt_names field in validation context is not supported on the server: %v", common)
	}
	sc.SubjectAltNameMatchers = matchers
	return sc, nil
}

// circuitBreakersFromCluster extracts the circuit breakers configuration from
// the received cluster resource. Returns nil if no CircuitBreakers or no
// Thresholds in CircuitBreakers.
func circuitBreakersFromCluster(cluster *v3clusterpb.Cluster) *uint32 {
	for _, threshold := range cluster.GetCircuitBreakers().GetThresholds() {
		if threshold.GetPriority() != v3corepb.RoutingPriority_DEFAULT {
			continue
		}
		maxRequestsPb := threshold.GetMaxRequests()
		if maxRequestsPb == nil {
			return nil
		}
		maxRequests := maxRequestsPb.GetValue()
		return &maxRequests
	}
	return nil
}
