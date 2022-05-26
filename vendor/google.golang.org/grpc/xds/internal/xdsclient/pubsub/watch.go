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

package pubsub

import (
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc/internal/pretty"
	"google.golang.org/grpc/xds/internal/xdsclient/xdsresource"
)

type watchInfoState int

const (
	watchInfoStateStarted watchInfoState = iota
	watchInfoStateRespReceived
	watchInfoStateTimeout
	watchInfoStateCanceled
)

// watchInfo holds all the information from a watch() call.
type watchInfo struct {
	c      *Pubsub
	rType  xdsresource.ResourceType
	target string

	ldsCallback func(xdsresource.ListenerUpdate, error)
	rdsCallback func(xdsresource.RouteConfigUpdate, error)
	cdsCallback func(xdsresource.ClusterUpdate, error)
	edsCallback func(xdsresource.EndpointsUpdate, error)

	expiryTimer *time.Timer

	// mu protects state, and c.scheduleCallback().
	// - No callback should be scheduled after watchInfo is canceled.
	// - No timeout error should be scheduled after watchInfo is resp received.
	mu    sync.Mutex
	state watchInfoState
}

func (wi *watchInfo) newUpdate(update interface{}) {
	wi.mu.Lock()
	defer wi.mu.Unlock()
	if wi.state == watchInfoStateCanceled {
		return
	}
	wi.state = watchInfoStateRespReceived
	wi.expiryTimer.Stop()
	wi.c.scheduleCallback(wi, update, nil)
}

func (wi *watchInfo) newError(err error) {
	wi.mu.Lock()
	defer wi.mu.Unlock()
	if wi.state == watchInfoStateCanceled {
		return
	}
	wi.state = watchInfoStateRespReceived
	wi.expiryTimer.Stop()
	wi.sendErrorLocked(err)
}

func (wi *watchInfo) resourceNotFound() {
	wi.mu.Lock()
	defer wi.mu.Unlock()
	if wi.state == watchInfoStateCanceled {
		return
	}
	wi.state = watchInfoStateRespReceived
	wi.expiryTimer.Stop()
	wi.sendErrorLocked(xdsresource.NewErrorf(xdsresource.ErrorTypeResourceNotFound, "xds: %v target %s not found in received response", wi.rType, wi.target))
}

func (wi *watchInfo) timeout() {
	wi.mu.Lock()
	defer wi.mu.Unlock()
	if wi.state == watchInfoStateCanceled || wi.state == watchInfoStateRespReceived {
		return
	}
	wi.state = watchInfoStateTimeout
	wi.sendErrorLocked(fmt.Errorf("xds: %v target %s not found, watcher timeout", wi.rType, wi.target))
}

// Caller must hold wi.mu.
func (wi *watchInfo) sendErrorLocked(err error) {
	var (
		u interface{}
	)
	switch wi.rType {
	case xdsresource.ListenerResource:
		u = xdsresource.ListenerUpdate{}
	case xdsresource.RouteConfigResource:
		u = xdsresource.RouteConfigUpdate{}
	case xdsresource.ClusterResource:
		u = xdsresource.ClusterUpdate{}
	case xdsresource.EndpointsResource:
		u = xdsresource.EndpointsUpdate{}
	}
	wi.c.scheduleCallback(wi, u, err)
}

func (wi *watchInfo) cancel() {
	wi.mu.Lock()
	defer wi.mu.Unlock()
	if wi.state == watchInfoStateCanceled {
		return
	}
	wi.expiryTimer.Stop()
	wi.state = watchInfoStateCanceled
}

func (pb *Pubsub) watch(wi *watchInfo) (first bool, cancel func() bool) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.logger.Debugf("new watch for type %v, resource name %v", wi.rType, wi.target)
	var (
		watchers map[string]map[*watchInfo]bool
		mds      map[string]xdsresource.UpdateMetadata
	)
	switch wi.rType {
	case xdsresource.ListenerResource:
		watchers = pb.ldsWatchers
		mds = pb.ldsMD
	case xdsresource.RouteConfigResource:
		watchers = pb.rdsWatchers
		mds = pb.rdsMD
	case xdsresource.ClusterResource:
		watchers = pb.cdsWatchers
		mds = pb.cdsMD
	case xdsresource.EndpointsResource:
		watchers = pb.edsWatchers
		mds = pb.edsMD
	default:
		pb.logger.Errorf("unknown watch type: %v", wi.rType)
		return false, nil
	}

	var firstWatcher bool
	resourceName := wi.target
	s, ok := watchers[wi.target]
	if !ok {
		// If this is a new watcher, will ask lower level to send a new request
		// with the resource name.
		//
		// If this (type+name) is already being watched, will not notify the
		// underlying versioned apiClient.
		pb.logger.Debugf("first watch for type %v, resource name %v, will send a new xDS request", wi.rType, wi.target)
		s = make(map[*watchInfo]bool)
		watchers[resourceName] = s
		mds[resourceName] = xdsresource.UpdateMetadata{Status: xdsresource.ServiceStatusRequested}
		firstWatcher = true
	}
	// No matter what, add the new watcher to the set, so it's callback will be
	// call for new responses.
	s[wi] = true

	// If the resource is in cache, call the callback with the value.
	switch wi.rType {
	case xdsresource.ListenerResource:
		if v, ok := pb.ldsCache[resourceName]; ok {
			pb.logger.Debugf("LDS resource with name %v found in cache: %+v", wi.target, pretty.ToJSON(v))
			wi.newUpdate(v)
		}
	case xdsresource.RouteConfigResource:
		if v, ok := pb.rdsCache[resourceName]; ok {
			pb.logger.Debugf("RDS resource with name %v found in cache: %+v", wi.target, pretty.ToJSON(v))
			wi.newUpdate(v)
		}
	case xdsresource.ClusterResource:
		if v, ok := pb.cdsCache[resourceName]; ok {
			pb.logger.Debugf("CDS resource with name %v found in cache: %+v", wi.target, pretty.ToJSON(v))
			wi.newUpdate(v)
		}
	case xdsresource.EndpointsResource:
		if v, ok := pb.edsCache[resourceName]; ok {
			pb.logger.Debugf("EDS resource with name %v found in cache: %+v", wi.target, pretty.ToJSON(v))
			wi.newUpdate(v)
		}
	}

	return firstWatcher, func() bool {
		pb.logger.Debugf("watch for type %v, resource name %v canceled", wi.rType, wi.target)
		wi.cancel()
		pb.mu.Lock()
		defer pb.mu.Unlock()
		var lastWatcher bool
		if s := watchers[resourceName]; s != nil {
			// Remove this watcher, so it's callback will not be called in the
			// future.
			delete(s, wi)
			if len(s) == 0 {
				pb.logger.Debugf("last watch for type %v, resource name %v canceled, will send a new xDS request", wi.rType, wi.target)
				// If this was the last watcher, also tell xdsv2Client to stop
				// watching this resource.
				delete(watchers, resourceName)
				delete(mds, resourceName)
				lastWatcher = true
				// Remove the resource from cache. When a watch for this
				// resource is added later, it will trigger a xDS request with
				// resource names, and client will receive new xDS responses.
				switch wi.rType {
				case xdsresource.ListenerResource:
					delete(pb.ldsCache, resourceName)
				case xdsresource.RouteConfigResource:
					delete(pb.rdsCache, resourceName)
				case xdsresource.ClusterResource:
					delete(pb.cdsCache, resourceName)
				case xdsresource.EndpointsResource:
					delete(pb.edsCache, resourceName)
				}
			}
		}
		return lastWatcher
	}
}
