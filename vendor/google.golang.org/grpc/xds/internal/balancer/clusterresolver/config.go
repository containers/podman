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

package clusterresolver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/grpc/balancer/roundrobin"
	internalserviceconfig "google.golang.org/grpc/internal/serviceconfig"
	"google.golang.org/grpc/serviceconfig"
	"google.golang.org/grpc/xds/internal/balancer/ringhash"
)

// DiscoveryMechanismType is the type of discovery mechanism.
type DiscoveryMechanismType int

const (
	// DiscoveryMechanismTypeEDS is eds.
	DiscoveryMechanismTypeEDS DiscoveryMechanismType = iota // `json:"EDS"`
	// DiscoveryMechanismTypeLogicalDNS is DNS.
	DiscoveryMechanismTypeLogicalDNS // `json:"LOGICAL_DNS"`
)

// MarshalJSON marshals a DiscoveryMechanismType to a quoted json string.
//
// This is necessary to handle enum (as strings) from JSON.
//
// Note that this needs to be defined on the type not pointer, otherwise the
// variables of this type will marshal to int not string.
func (t DiscoveryMechanismType) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	switch t {
	case DiscoveryMechanismTypeEDS:
		buffer.WriteString("EDS")
	case DiscoveryMechanismTypeLogicalDNS:
		buffer.WriteString("LOGICAL_DNS")
	}
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmarshals a quoted json string to the DiscoveryMechanismType.
func (t *DiscoveryMechanismType) UnmarshalJSON(b []byte) error {
	var s string
	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}
	switch s {
	case "EDS":
		*t = DiscoveryMechanismTypeEDS
	case "LOGICAL_DNS":
		*t = DiscoveryMechanismTypeLogicalDNS
	default:
		return fmt.Errorf("unable to unmarshal string %q to type DiscoveryMechanismType", s)
	}
	return nil
}

// DiscoveryMechanism is the discovery mechanism, can be either EDS or DNS.
//
// For DNS, the ClientConn target will be used for name resolution.
//
// For EDS, if EDSServiceName is not empty, it will be used for watching. If
// EDSServiceName is empty, Cluster will be used.
type DiscoveryMechanism struct {
	// Cluster is the cluster name.
	Cluster string `json:"cluster,omitempty"`
	// LoadReportingServerName is the LRS server to send load reports to. If
	// not present, load reporting will be disabled. If set to the empty string,
	// load reporting will be sent to the same server that we obtained CDS data
	// from.
	LoadReportingServerName *string `json:"lrsLoadReportingServerName,omitempty"`
	// MaxConcurrentRequests is the maximum number of outstanding requests can
	// be made to the upstream cluster. Default is 1024.
	MaxConcurrentRequests *uint32 `json:"maxConcurrentRequests,omitempty"`
	// Type is the discovery mechanism type.
	Type DiscoveryMechanismType `json:"type,omitempty"`
	// EDSServiceName is the EDS service name, as returned in CDS. May be unset
	// if not specified in CDS. For type EDS only.
	//
	// This is used for EDS watch if set. If unset, Cluster is used for EDS
	// watch.
	EDSServiceName string `json:"edsServiceName,omitempty"`
	// DNSHostname is the DNS name to resolve in "host:port" form. For type
	// LOGICAL_DNS only.
	DNSHostname string `json:"dnsHostname,omitempty"`
}

// Equal returns whether the DiscoveryMechanism is the same with the parameter.
func (dm DiscoveryMechanism) Equal(b DiscoveryMechanism) bool {
	switch {
	case dm.Cluster != b.Cluster:
		return false
	case !equalStringP(dm.LoadReportingServerName, b.LoadReportingServerName):
		return false
	case !equalUint32P(dm.MaxConcurrentRequests, b.MaxConcurrentRequests):
		return false
	case dm.Type != b.Type:
		return false
	case dm.EDSServiceName != b.EDSServiceName:
		return false
	case dm.DNSHostname != b.DNSHostname:
		return false
	}
	return true
}

func equalStringP(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func equalUint32P(a, b *uint32) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// LBConfig is the config for cluster resolver balancer.
type LBConfig struct {
	serviceconfig.LoadBalancingConfig `json:"-"`
	// DiscoveryMechanisms is an ordered list of discovery mechanisms.
	//
	// Must have at least one element. Results from each discovery mechanism are
	// concatenated together in successive priorities.
	DiscoveryMechanisms []DiscoveryMechanism `json:"discoveryMechanisms,omitempty"`

	// XDSLBPolicy specifies the policy for locality picking and endpoint picking.
	//
	// Note that it's not normal balancing policy, and it can only be either
	// ROUND_ROBIN or RING_HASH.
	//
	// For ROUND_ROBIN, the policy name will be "ROUND_ROBIN", and the config
	// will be empty. This sets the locality-picking policy to weighted_target
	// and the endpoint-picking policy to round_robin.
	//
	// For RING_HASH, the policy name will be "RING_HASH", and the config will
	// be lb config for the ring_hash_experimental LB Policy. ring_hash policy
	// is responsible for both locality picking and endpoint picking.
	XDSLBPolicy *internalserviceconfig.BalancerConfig `json:"xdsLbPolicy,omitempty"`
}

const (
	rrName = roundrobin.Name
	rhName = ringhash.Name
)

func parseConfig(c json.RawMessage) (*LBConfig, error) {
	var cfg LBConfig
	if err := json.Unmarshal(c, &cfg); err != nil {
		return nil, err
	}
	if lbp := cfg.XDSLBPolicy; lbp != nil && !strings.EqualFold(lbp.Name, rrName) && !strings.EqualFold(lbp.Name, rhName) {
		return nil, fmt.Errorf("unsupported child policy with name %q, not one of {%q,%q}", lbp.Name, rrName, rhName)
	}
	return &cfg, nil
}
