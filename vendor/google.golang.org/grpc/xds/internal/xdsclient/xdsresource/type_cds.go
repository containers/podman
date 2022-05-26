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

import "google.golang.org/protobuf/types/known/anypb"

// ClusterType is the type of cluster from a received CDS response.
type ClusterType int

const (
	// ClusterTypeEDS represents the EDS cluster type, which will delegate endpoint
	// discovery to the management server.
	ClusterTypeEDS ClusterType = iota
	// ClusterTypeLogicalDNS represents the Logical DNS cluster type, which essentially
	// maps to the gRPC behavior of using the DNS resolver with pick_first LB policy.
	ClusterTypeLogicalDNS
	// ClusterTypeAggregate represents the Aggregate Cluster type, which provides a
	// prioritized list of clusters to use. It is used for failover between clusters
	// with a different configuration.
	ClusterTypeAggregate
)

// ClusterLBPolicyRingHash represents ring_hash lb policy, and also contains its
// config.
type ClusterLBPolicyRingHash struct {
	MinimumRingSize uint64
	MaximumRingSize uint64
}

// ClusterUpdate contains information from a received CDS response, which is of
// interest to the registered CDS watcher.
type ClusterUpdate struct {
	ClusterType ClusterType
	// ClusterName is the clusterName being watched for through CDS.
	ClusterName string
	// EDSServiceName is an optional name for EDS. If it's not set, the balancer
	// should watch ClusterName for the EDS resources.
	EDSServiceName string
	// EnableLRS indicates whether or not load should be reported through LRS.
	EnableLRS bool
	// SecurityCfg contains security configuration sent by the control plane.
	SecurityCfg *SecurityConfig
	// MaxRequests for circuit breaking, if any (otherwise nil).
	MaxRequests *uint32
	// DNSHostName is used only for cluster type DNS. It's the DNS name to
	// resolve in "host:port" form
	DNSHostName string
	// PrioritizedClusterNames is used only for cluster type aggregate. It represents
	// a prioritized list of cluster names.
	PrioritizedClusterNames []string

	// LBPolicy is the lb policy for this cluster.
	//
	// This only support round_robin and ring_hash.
	// - if it's nil, the lb policy is round_robin
	// - if it's not nil, the lb policy is ring_hash, the this field has the config.
	//
	// When we add more support policies, this can be made an interface, and
	// will be set to different types based on the policy type.
	LBPolicy *ClusterLBPolicyRingHash

	// Raw is the resource from the xds response.
	Raw *anypb.Any
}

// ClusterUpdateErrTuple is a tuple with the update and error. It contains the
// results from unmarshal functions. It's used to pass unmarshal results of
// multiple resources together, e.g. in maps like `map[string]{Update,error}`.
type ClusterUpdateErrTuple struct {
	Update ClusterUpdate
	Err    error
}
