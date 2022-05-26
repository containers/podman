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

// Package pubsub implements a utility type to maintain resource watchers and
// the updates.
//
// This package is designed to work with the xds resources. It could be made a
// general system that works with all types.
package pubsub

import (
	"sync"
	"time"

	"google.golang.org/grpc/internal/buffer"
	"google.golang.org/grpc/internal/grpclog"
	"google.golang.org/grpc/internal/grpcsync"
	"google.golang.org/grpc/xds/internal/xdsclient/xdsresource"
)

// Pubsub maintains resource watchers and resource updates.
//
// There can be multiple watchers for the same resource. An update to a resource
// triggers updates to all the existing watchers. Watchers can be canceled at
// any time.
type Pubsub struct {
	done               *grpcsync.Event
	logger             *grpclog.PrefixLogger
	watchExpiryTimeout time.Duration

	updateCh *buffer.Unbounded // chan *watcherInfoWithUpdate
	// All the following maps are to keep the updates/metadata in a cache.
	mu          sync.Mutex
	ldsWatchers map[string]map[*watchInfo]bool
	ldsCache    map[string]xdsresource.ListenerUpdate
	ldsMD       map[string]xdsresource.UpdateMetadata
	rdsWatchers map[string]map[*watchInfo]bool
	rdsCache    map[string]xdsresource.RouteConfigUpdate
	rdsMD       map[string]xdsresource.UpdateMetadata
	cdsWatchers map[string]map[*watchInfo]bool
	cdsCache    map[string]xdsresource.ClusterUpdate
	cdsMD       map[string]xdsresource.UpdateMetadata
	edsWatchers map[string]map[*watchInfo]bool
	edsCache    map[string]xdsresource.EndpointsUpdate
	edsMD       map[string]xdsresource.UpdateMetadata
}

// New creates a new Pubsub.
func New(watchExpiryTimeout time.Duration, logger *grpclog.PrefixLogger) *Pubsub {
	pb := &Pubsub{
		done:               grpcsync.NewEvent(),
		logger:             logger,
		watchExpiryTimeout: watchExpiryTimeout,

		updateCh:    buffer.NewUnbounded(),
		ldsWatchers: make(map[string]map[*watchInfo]bool),
		ldsCache:    make(map[string]xdsresource.ListenerUpdate),
		ldsMD:       make(map[string]xdsresource.UpdateMetadata),
		rdsWatchers: make(map[string]map[*watchInfo]bool),
		rdsCache:    make(map[string]xdsresource.RouteConfigUpdate),
		rdsMD:       make(map[string]xdsresource.UpdateMetadata),
		cdsWatchers: make(map[string]map[*watchInfo]bool),
		cdsCache:    make(map[string]xdsresource.ClusterUpdate),
		cdsMD:       make(map[string]xdsresource.UpdateMetadata),
		edsWatchers: make(map[string]map[*watchInfo]bool),
		edsCache:    make(map[string]xdsresource.EndpointsUpdate),
		edsMD:       make(map[string]xdsresource.UpdateMetadata),
	}
	go pb.run()
	return pb
}

// WatchListener registers a watcher for the LDS resource.
//
// It also returns whether this is the first watch for this resource.
func (pb *Pubsub) WatchListener(serviceName string, cb func(xdsresource.ListenerUpdate, error)) (first bool, cancel func() bool) {
	wi := &watchInfo{
		c:           pb,
		rType:       xdsresource.ListenerResource,
		target:      serviceName,
		ldsCallback: cb,
	}

	wi.expiryTimer = time.AfterFunc(pb.watchExpiryTimeout, func() {
		wi.timeout()
	})
	return pb.watch(wi)
}

// WatchRouteConfig register a watcher for the RDS resource.
//
// It also returns whether this is the first watch for this resource.
func (pb *Pubsub) WatchRouteConfig(routeName string, cb func(xdsresource.RouteConfigUpdate, error)) (first bool, cancel func() bool) {
	wi := &watchInfo{
		c:           pb,
		rType:       xdsresource.RouteConfigResource,
		target:      routeName,
		rdsCallback: cb,
	}

	wi.expiryTimer = time.AfterFunc(pb.watchExpiryTimeout, func() {
		wi.timeout()
	})
	return pb.watch(wi)
}

// WatchCluster register a watcher for the CDS resource.
//
// It also returns whether this is the first watch for this resource.
func (pb *Pubsub) WatchCluster(clusterName string, cb func(xdsresource.ClusterUpdate, error)) (first bool, cancel func() bool) {
	wi := &watchInfo{
		c:           pb,
		rType:       xdsresource.ClusterResource,
		target:      clusterName,
		cdsCallback: cb,
	}

	wi.expiryTimer = time.AfterFunc(pb.watchExpiryTimeout, func() {
		wi.timeout()
	})
	return pb.watch(wi)
}

// WatchEndpoints registers a watcher for the EDS resource.
//
// It also returns whether this is the first watch for this resource.
func (pb *Pubsub) WatchEndpoints(clusterName string, cb func(xdsresource.EndpointsUpdate, error)) (first bool, cancel func() bool) {
	wi := &watchInfo{
		c:           pb,
		rType:       xdsresource.EndpointsResource,
		target:      clusterName,
		edsCallback: cb,
	}

	wi.expiryTimer = time.AfterFunc(pb.watchExpiryTimeout, func() {
		wi.timeout()
	})
	return pb.watch(wi)
}

// Close closes the pubsub.
func (pb *Pubsub) Close() {
	if pb.done.HasFired() {
		return
	}
	pb.done.Fire()
}

// run is a goroutine for all the callbacks.
//
// Callback can be called in watch(), if an item is found in cache. Without this
// goroutine, the callback will be called inline, which might cause a deadlock
// in user's code. Callbacks also cannot be simple `go callback()` because the
// order matters.
func (pb *Pubsub) run() {
	for {
		select {
		case t := <-pb.updateCh.Get():
			pb.updateCh.Load()
			if pb.done.HasFired() {
				return
			}
			pb.callCallback(t.(*watcherInfoWithUpdate))
		case <-pb.done.Done():
			return
		}
	}
}
