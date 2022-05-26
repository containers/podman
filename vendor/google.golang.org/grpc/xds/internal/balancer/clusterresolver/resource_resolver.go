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
 *
 */

package clusterresolver

import (
	"sync"

	"google.golang.org/grpc/xds/internal/xdsclient"
	"google.golang.org/grpc/xds/internal/xdsclient/xdsresource"
)

// resourceUpdate is a combined update from all the resources, in the order of
// priority. For example, it can be {EDS, EDS, DNS}.
type resourceUpdate struct {
	priorities []priorityConfig
	err        error
}

type discoveryMechanism interface {
	lastUpdate() (interface{}, bool)
	resolveNow()
	stop()
}

// discoveryMechanismKey is {type+resource_name}, it's used as the map key, so
// that the same resource resolver can be reused (e.g. when there are two
// mechanisms, both for the same EDS resource, but has different circuit
// breaking config.
type discoveryMechanismKey struct {
	typ  DiscoveryMechanismType
	name string
}

// resolverMechanismTuple is needed to keep the resolver and the discovery
// mechanism together, because resolvers can be shared. And we need the
// mechanism for fields like circuit breaking, LRS etc when generating the
// balancer config.
type resolverMechanismTuple struct {
	dm    DiscoveryMechanism
	dmKey discoveryMechanismKey
	r     discoveryMechanism
}

type resourceResolver struct {
	parent        *clusterResolverBalancer
	updateChannel chan *resourceUpdate

	// mu protects the slice and map, and content of the resolvers in the slice.
	mu          sync.Mutex
	mechanisms  []DiscoveryMechanism
	children    []resolverMechanismTuple
	childrenMap map[discoveryMechanismKey]discoveryMechanism
}

func newResourceResolver(parent *clusterResolverBalancer) *resourceResolver {
	return &resourceResolver{
		parent:        parent,
		updateChannel: make(chan *resourceUpdate, 1),
		childrenMap:   make(map[discoveryMechanismKey]discoveryMechanism),
	}
}

func equalDiscoveryMechanisms(a, b []DiscoveryMechanism) bool {
	if len(a) != len(b) {
		return false
	}
	for i, aa := range a {
		bb := b[i]
		if !aa.Equal(bb) {
			return false
		}
	}
	return true
}

func (rr *resourceResolver) updateMechanisms(mechanisms []DiscoveryMechanism) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if equalDiscoveryMechanisms(rr.mechanisms, mechanisms) {
		return
	}
	rr.mechanisms = mechanisms
	rr.children = make([]resolverMechanismTuple, len(mechanisms))
	newDMs := make(map[discoveryMechanismKey]bool)

	// Start one watch for each new discover mechanism {type+resource_name}.
	for i, dm := range mechanisms {
		switch dm.Type {
		case DiscoveryMechanismTypeEDS:
			// If EDSServiceName is not set, use the cluster name as EDS service
			// name to watch.
			nameToWatch := dm.EDSServiceName
			if nameToWatch == "" {
				nameToWatch = dm.Cluster
			}
			dmKey := discoveryMechanismKey{typ: dm.Type, name: nameToWatch}
			newDMs[dmKey] = true

			r := rr.childrenMap[dmKey]
			if r == nil {
				r = newEDSResolver(nameToWatch, rr.parent.xdsClient, rr)
				rr.childrenMap[dmKey] = r
			}
			rr.children[i] = resolverMechanismTuple{dm: dm, dmKey: dmKey, r: r}
		case DiscoveryMechanismTypeLogicalDNS:
			// Name to resolve in DNS is the hostname, not the ClientConn
			// target.
			dmKey := discoveryMechanismKey{typ: dm.Type, name: dm.DNSHostname}
			newDMs[dmKey] = true

			r := rr.childrenMap[dmKey]
			if r == nil {
				r = newDNSResolver(dm.DNSHostname, rr)
				rr.childrenMap[dmKey] = r
			}
			rr.children[i] = resolverMechanismTuple{dm: dm, dmKey: dmKey, r: r}
		}
	}
	// Stop the resources that were removed.
	for dm, r := range rr.childrenMap {
		if !newDMs[dm] {
			delete(rr.childrenMap, dm)
			r.stop()
		}
	}
	// Regenerate even if there's no change in discovery mechanism, in case
	// priority order changed.
	rr.generate()
}

// resolveNow is typically called to trigger re-resolve of DNS. The EDS
// resolveNow() is a noop.
func (rr *resourceResolver) resolveNow() {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	for _, r := range rr.childrenMap {
		r.resolveNow()
	}
}

func (rr *resourceResolver) stop() {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	for dm, r := range rr.childrenMap {
		delete(rr.childrenMap, dm)
		r.stop()
	}
	rr.mechanisms = nil
	rr.children = nil
}

// generate collects all the updates from all the resolvers, and push the
// combined result into the update channel. It only pushes the update when all
// the child resolvers have received at least one update, otherwise it will
// wait.
//
// caller must hold rr.mu.
func (rr *resourceResolver) generate() {
	var ret []priorityConfig
	for _, rDM := range rr.children {
		r, ok := rr.childrenMap[rDM.dmKey]
		if !ok {
			rr.parent.logger.Infof("resolver for %+v not found, should never happen", rDM.dmKey)
			continue
		}

		u, ok := r.lastUpdate()
		if !ok {
			// Don't send updates to parent until all resolvers have update to
			// send.
			return
		}
		switch uu := u.(type) {
		case xdsresource.EndpointsUpdate:
			ret = append(ret, priorityConfig{mechanism: rDM.dm, edsResp: uu})
		case []string:
			ret = append(ret, priorityConfig{mechanism: rDM.dm, addresses: uu})
		}
	}
	select {
	case <-rr.updateChannel:
	default:
	}
	rr.updateChannel <- &resourceUpdate{priorities: ret}
}

type edsDiscoveryMechanism struct {
	cancel func()

	update         xdsresource.EndpointsUpdate
	updateReceived bool
}

func (er *edsDiscoveryMechanism) lastUpdate() (interface{}, bool) {
	if !er.updateReceived {
		return nil, false
	}
	return er.update, true
}

func (er *edsDiscoveryMechanism) resolveNow() {
}

func (er *edsDiscoveryMechanism) stop() {
	er.cancel()
}

// newEDSResolver starts the EDS watch on the given xds client.
func newEDSResolver(nameToWatch string, xdsc xdsclient.XDSClient, topLevelResolver *resourceResolver) *edsDiscoveryMechanism {
	ret := &edsDiscoveryMechanism{}
	topLevelResolver.parent.logger.Infof("EDS watch started on %v", nameToWatch)
	cancel := xdsc.WatchEndpoints(nameToWatch, func(update xdsresource.EndpointsUpdate, err error) {
		topLevelResolver.mu.Lock()
		defer topLevelResolver.mu.Unlock()
		if err != nil {
			select {
			case <-topLevelResolver.updateChannel:
			default:
			}
			topLevelResolver.updateChannel <- &resourceUpdate{err: err}
			return
		}
		ret.update = update
		ret.updateReceived = true
		topLevelResolver.generate()
	})
	ret.cancel = func() {
		topLevelResolver.parent.logger.Infof("EDS watch canceled on %v", nameToWatch)
		cancel()
	}
	return ret
}
