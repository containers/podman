/*
 *
 * Copyright 2020 gRPC authors.
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
 *
 */

package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"math/bits"
	"strings"
	"sync/atomic"
	"time"

	xxhash "github.com/cespare/xxhash/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/internal/envconfig"
	"google.golang.org/grpc/internal/grpcrand"
	iresolver "google.golang.org/grpc/internal/resolver"
	"google.golang.org/grpc/internal/serviceconfig"
	"google.golang.org/grpc/internal/wrr"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/xds/internal/balancer/clustermanager"
	"google.golang.org/grpc/xds/internal/balancer/ringhash"
	"google.golang.org/grpc/xds/internal/httpfilter"
	"google.golang.org/grpc/xds/internal/httpfilter/router"
	"google.golang.org/grpc/xds/internal/xdsclient/xdsresource"
)

const (
	cdsName                      = "cds_experimental"
	xdsClusterManagerName        = "xds_cluster_manager_experimental"
	clusterPrefix                = "cluster:"
	clusterSpecifierPluginPrefix = "cluster_specifier_plugin:"
)

type serviceConfig struct {
	LoadBalancingConfig balancerConfig `json:"loadBalancingConfig"`
}

type balancerConfig []map[string]interface{}

func newBalancerConfig(name string, config interface{}) balancerConfig {
	return []map[string]interface{}{{name: config}}
}

type cdsBalancerConfig struct {
	Cluster string `json:"cluster"`
}

type xdsChildConfig struct {
	ChildPolicy balancerConfig `json:"childPolicy"`
}

type xdsClusterManagerConfig struct {
	Children map[string]xdsChildConfig `json:"children"`
}

// pruneActiveClusters deletes entries in r.activeClusters with zero
// references.
func (r *xdsResolver) pruneActiveClusters() {
	for cluster, ci := range r.activeClusters {
		if atomic.LoadInt32(&ci.refCount) == 0 {
			delete(r.activeClusters, cluster)
		}
	}
}

// serviceConfigJSON produces a service config in JSON format representing all
// the clusters referenced in activeClusters.  This includes clusters with zero
// references, so they must be pruned first.
func serviceConfigJSON(activeClusters map[string]*clusterInfo) ([]byte, error) {
	// Generate children (all entries in activeClusters).
	children := make(map[string]xdsChildConfig)
	for cluster, ci := range activeClusters {
		children[cluster] = ci.cfg
	}

	sc := serviceConfig{
		LoadBalancingConfig: newBalancerConfig(
			xdsClusterManagerName, xdsClusterManagerConfig{Children: children},
		),
	}

	bs, err := json.Marshal(sc)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json: %v", err)
	}
	return bs, nil
}

type virtualHost struct {
	// map from filter name to its config
	httpFilterConfigOverride map[string]httpfilter.FilterConfig
	// retry policy present in virtual host
	retryConfig *xdsresource.RetryConfig
}

// routeCluster holds information about a cluster as referenced by a route.
type routeCluster struct {
	name string
	// map from filter name to its config
	httpFilterConfigOverride map[string]httpfilter.FilterConfig
}

type route struct {
	m                 *xdsresource.CompositeMatcher // converted from route matchers
	clusters          wrr.WRR                       // holds *routeCluster entries
	maxStreamDuration time.Duration
	// map from filter name to its config
	httpFilterConfigOverride map[string]httpfilter.FilterConfig
	retryConfig              *xdsresource.RetryConfig
	hashPolicies             []*xdsresource.HashPolicy
}

func (r route) String() string {
	return fmt.Sprintf("%s -> { clusters: %v, maxStreamDuration: %v }", r.m.String(), r.clusters, r.maxStreamDuration)
}

type configSelector struct {
	r                *xdsResolver
	virtualHost      virtualHost
	routes           []route
	clusters         map[string]*clusterInfo
	httpFilterConfig []xdsresource.HTTPFilter
}

var errNoMatchedRouteFound = status.Errorf(codes.Unavailable, "no matched route was found")

func (cs *configSelector) SelectConfig(rpcInfo iresolver.RPCInfo) (*iresolver.RPCConfig, error) {
	if cs == nil {
		return nil, status.Errorf(codes.Unavailable, "no valid clusters")
	}
	var rt *route
	// Loop through routes in order and select first match.
	for _, r := range cs.routes {
		if r.m.Match(rpcInfo) {
			rt = &r
			break
		}
	}
	if rt == nil || rt.clusters == nil {
		return nil, errNoMatchedRouteFound
	}

	cluster, ok := rt.clusters.Next().(*routeCluster)
	if !ok {
		return nil, status.Errorf(codes.Internal, "error retrieving cluster for match: %v (%T)", cluster, cluster)
	}

	// Add a ref to the selected cluster, as this RPC needs this cluster until
	// it is committed.
	ref := &cs.clusters[cluster.name].refCount
	atomic.AddInt32(ref, 1)

	interceptor, err := cs.newInterceptor(rt, cluster)
	if err != nil {
		return nil, err
	}

	lbCtx := clustermanager.SetPickedCluster(rpcInfo.Context, cluster.name)
	// Request Hashes are only applicable for a Ring Hash LB.
	if envconfig.XDSRingHash {
		lbCtx = ringhash.SetRequestHash(lbCtx, cs.generateHash(rpcInfo, rt.hashPolicies))
	}

	config := &iresolver.RPCConfig{
		// Communicate to the LB policy the chosen cluster and request hash, if Ring Hash LB policy.
		Context: lbCtx,
		OnCommitted: func() {
			// When the RPC is committed, the cluster is no longer required.
			// Decrease its ref.
			if v := atomic.AddInt32(ref, -1); v == 0 {
				// This entry will be removed from activeClusters when
				// producing the service config for the empty update.
				select {
				case cs.r.updateCh <- suWithError{emptyUpdate: true}:
				default:
				}
			}
		},
		Interceptor: interceptor,
	}

	if rt.maxStreamDuration != 0 {
		config.MethodConfig.Timeout = &rt.maxStreamDuration
	}
	if rt.retryConfig != nil {
		config.MethodConfig.RetryPolicy = retryConfigToPolicy(rt.retryConfig)
	} else if cs.virtualHost.retryConfig != nil {
		config.MethodConfig.RetryPolicy = retryConfigToPolicy(cs.virtualHost.retryConfig)
	}

	return config, nil
}

func retryConfigToPolicy(config *xdsresource.RetryConfig) *serviceconfig.RetryPolicy {
	return &serviceconfig.RetryPolicy{
		MaxAttempts:          int(config.NumRetries) + 1,
		InitialBackoff:       config.RetryBackoff.BaseInterval,
		MaxBackoff:           config.RetryBackoff.MaxInterval,
		BackoffMultiplier:    2,
		RetryableStatusCodes: config.RetryOn,
	}
}

func (cs *configSelector) generateHash(rpcInfo iresolver.RPCInfo, hashPolicies []*xdsresource.HashPolicy) uint64 {
	var hash uint64
	var generatedHash bool
	for _, policy := range hashPolicies {
		var policyHash uint64
		var generatedPolicyHash bool
		switch policy.HashPolicyType {
		case xdsresource.HashPolicyTypeHeader:
			md, ok := metadata.FromOutgoingContext(rpcInfo.Context)
			if !ok {
				continue
			}
			values := md.Get(policy.HeaderName)
			// If the header isn't present, no-op.
			if len(values) == 0 {
				continue
			}
			joinedValues := strings.Join(values, ",")
			if policy.Regex != nil {
				joinedValues = policy.Regex.ReplaceAllString(joinedValues, policy.RegexSubstitution)
			}
			policyHash = xxhash.Sum64String(joinedValues)
			generatedHash = true
			generatedPolicyHash = true
		case xdsresource.HashPolicyTypeChannelID:
			// Hash the ClientConn pointer which logically uniquely
			// identifies the client.
			policyHash = xxhash.Sum64String(fmt.Sprintf("%p", &cs.r.cc))
			generatedHash = true
			generatedPolicyHash = true
		}

		// Deterministically combine the hash policies. Rotating prevents
		// duplicate hash policies from cancelling each other out and preserves
		// the 64 bits of entropy.
		if generatedPolicyHash {
			hash = bits.RotateLeft64(hash, 1)
			hash = hash ^ policyHash
		}

		// If terminal policy and a hash has already been generated, ignore the
		// rest of the policies and use that hash already generated.
		if policy.Terminal && generatedHash {
			break
		}
	}

	if generatedHash {
		return hash
	}
	// If no generated hash return a random long. In the grand scheme of things
	// this logically will map to choosing a random backend to route request to.
	return grpcrand.Uint64()
}

func (cs *configSelector) newInterceptor(rt *route, cluster *routeCluster) (iresolver.ClientInterceptor, error) {
	if len(cs.httpFilterConfig) == 0 {
		return nil, nil
	}
	interceptors := make([]iresolver.ClientInterceptor, 0, len(cs.httpFilterConfig))
	for _, filter := range cs.httpFilterConfig {
		if router.IsRouterFilter(filter.Filter) {
			// Ignore any filters after the router filter.  The router itself
			// is currently a nop.
			return &interceptorList{interceptors: interceptors}, nil
		}
		override := cluster.httpFilterConfigOverride[filter.Name] // cluster is highest priority
		if override == nil {
			override = rt.httpFilterConfigOverride[filter.Name] // route is second priority
		}
		if override == nil {
			override = cs.virtualHost.httpFilterConfigOverride[filter.Name] // VH is third & lowest priority
		}
		ib, ok := filter.Filter.(httpfilter.ClientInterceptorBuilder)
		if !ok {
			// Should not happen if it passed xdsClient validation.
			return nil, fmt.Errorf("filter does not support use in client")
		}
		i, err := ib.BuildClientInterceptor(filter.Config, override)
		if err != nil {
			return nil, fmt.Errorf("error constructing filter: %v", err)
		}
		if i != nil {
			interceptors = append(interceptors, i)
		}
	}
	return nil, fmt.Errorf("error in xds config: no router filter present")
}

// stop decrements refs of all clusters referenced by this config selector.
func (cs *configSelector) stop() {
	// The resolver's old configSelector may be nil.  Handle that here.
	if cs == nil {
		return
	}
	// If any refs drop to zero, we'll need a service config update to delete
	// the cluster.
	needUpdate := false
	// Loops over cs.clusters, but these are pointers to entries in
	// activeClusters.
	for _, ci := range cs.clusters {
		if v := atomic.AddInt32(&ci.refCount, -1); v == 0 {
			needUpdate = true
		}
	}
	// We stop the old config selector immediately after sending a new config
	// selector; we need another update to delete clusters from the config (if
	// we don't have another update pending already).
	if needUpdate {
		select {
		case cs.r.updateCh <- suWithError{emptyUpdate: true}:
		default:
		}
	}
}

// A global for testing.
var newWRR = wrr.NewRandom

// newConfigSelector creates the config selector for su; may add entries to
// r.activeClusters for previously-unseen clusters.
func (r *xdsResolver) newConfigSelector(su serviceUpdate) (*configSelector, error) {
	cs := &configSelector{
		r: r,
		virtualHost: virtualHost{
			httpFilterConfigOverride: su.virtualHost.HTTPFilterConfigOverride,
			retryConfig:              su.virtualHost.RetryConfig,
		},
		routes:           make([]route, len(su.virtualHost.Routes)),
		clusters:         make(map[string]*clusterInfo),
		httpFilterConfig: su.ldsConfig.httpFilterConfig,
	}

	for i, rt := range su.virtualHost.Routes {
		clusters := newWRR()
		if rt.ClusterSpecifierPlugin != "" {
			clusterName := clusterSpecifierPluginPrefix + rt.ClusterSpecifierPlugin
			clusters.Add(&routeCluster{
				name: clusterName,
			}, 1)
			cs.initializeCluster(clusterName, xdsChildConfig{
				ChildPolicy: balancerConfig(su.clusterSpecifierPlugins[rt.ClusterSpecifierPlugin]),
			})
		} else {
			for cluster, wc := range rt.WeightedClusters {
				clusterName := clusterPrefix + cluster
				clusters.Add(&routeCluster{
					name:                     clusterName,
					httpFilterConfigOverride: wc.HTTPFilterConfigOverride,
				}, int64(wc.Weight))
				cs.initializeCluster(clusterName, xdsChildConfig{
					ChildPolicy: newBalancerConfig(cdsName, cdsBalancerConfig{Cluster: cluster}),
				})
			}
		}
		cs.routes[i].clusters = clusters

		var err error
		cs.routes[i].m, err = xdsresource.RouteToMatcher(rt)
		if err != nil {
			return nil, err
		}
		if rt.MaxStreamDuration == nil {
			cs.routes[i].maxStreamDuration = su.ldsConfig.maxStreamDuration
		} else {
			cs.routes[i].maxStreamDuration = *rt.MaxStreamDuration
		}

		cs.routes[i].httpFilterConfigOverride = rt.HTTPFilterConfigOverride
		cs.routes[i].retryConfig = rt.RetryConfig
		cs.routes[i].hashPolicies = rt.HashPolicies
	}

	// Account for this config selector's clusters.  Do this after no further
	// errors may occur.  Note: cs.clusters are pointers to entries in
	// activeClusters.
	for _, ci := range cs.clusters {
		atomic.AddInt32(&ci.refCount, 1)
	}

	return cs, nil
}

// initializeCluster initializes entries in cs.clusters map, creating entries in
// r.activeClusters as necessary.  Any created entries will have a ref count set
// to zero as their ref count will be incremented by incRefs.
func (cs *configSelector) initializeCluster(clusterName string, cfg xdsChildConfig) {
	ci := cs.r.activeClusters[clusterName]
	if ci == nil {
		ci = &clusterInfo{refCount: 0}
		cs.r.activeClusters[clusterName] = ci
	}
	cs.clusters[clusterName] = ci
	cs.clusters[clusterName].cfg = cfg
}

type clusterInfo struct {
	// number of references to this cluster; accessed atomically
	refCount int32
	// cfg is the child configuration for this cluster, containing either the
	// csp config or the cds cluster config.
	cfg xdsChildConfig
}

type interceptorList struct {
	interceptors []iresolver.ClientInterceptor
}

func (il *interceptorList) NewStream(ctx context.Context, ri iresolver.RPCInfo, done func(), newStream func(ctx context.Context, done func()) (iresolver.ClientStream, error)) (iresolver.ClientStream, error) {
	for i := len(il.interceptors) - 1; i >= 0; i-- {
		ns := newStream
		interceptor := il.interceptors[i]
		newStream = func(ctx context.Context, done func()) (iresolver.ClientStream, error) {
			return interceptor.NewStream(ctx, ri, done, ns)
		}
	}
	return newStream(ctx, func() {})
}
