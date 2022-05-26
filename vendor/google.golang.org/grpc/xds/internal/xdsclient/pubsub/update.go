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
	"google.golang.org/grpc/internal/pretty"
	"google.golang.org/grpc/xds/internal/xdsclient/xdsresource"
	"google.golang.org/protobuf/proto"
)

type watcherInfoWithUpdate struct {
	wi     *watchInfo
	update interface{}
	err    error
}

// scheduleCallback should only be called by methods of watchInfo, which checks
// for watcher states and maintain consistency.
func (pb *Pubsub) scheduleCallback(wi *watchInfo, update interface{}, err error) {
	pb.updateCh.Put(&watcherInfoWithUpdate{
		wi:     wi,
		update: update,
		err:    err,
	})
}

func (pb *Pubsub) callCallback(wiu *watcherInfoWithUpdate) {
	pb.mu.Lock()
	// Use a closure to capture the callback and type assertion, to save one
	// more switch case.
	//
	// The callback must be called without pb.mu. Otherwise if the callback calls
	// another watch() inline, it will cause a deadlock. This leaves a small
	// window that a watcher's callback could be called after the watcher is
	// canceled, and the user needs to take care of it.
	var ccb func()
	switch wiu.wi.rType {
	case xdsresource.ListenerResource:
		if s, ok := pb.ldsWatchers[wiu.wi.target]; ok && s[wiu.wi] {
			ccb = func() { wiu.wi.ldsCallback(wiu.update.(xdsresource.ListenerUpdate), wiu.err) }
		}
	case xdsresource.RouteConfigResource:
		if s, ok := pb.rdsWatchers[wiu.wi.target]; ok && s[wiu.wi] {
			ccb = func() { wiu.wi.rdsCallback(wiu.update.(xdsresource.RouteConfigUpdate), wiu.err) }
		}
	case xdsresource.ClusterResource:
		if s, ok := pb.cdsWatchers[wiu.wi.target]; ok && s[wiu.wi] {
			ccb = func() { wiu.wi.cdsCallback(wiu.update.(xdsresource.ClusterUpdate), wiu.err) }
		}
	case xdsresource.EndpointsResource:
		if s, ok := pb.edsWatchers[wiu.wi.target]; ok && s[wiu.wi] {
			ccb = func() { wiu.wi.edsCallback(wiu.update.(xdsresource.EndpointsUpdate), wiu.err) }
		}
	}
	pb.mu.Unlock()

	if ccb != nil {
		ccb()
	}
}

// NewListeners is called when there's a new LDS update.
func (pb *Pubsub) NewListeners(updates map[string]xdsresource.ListenerUpdateErrTuple, metadata xdsresource.UpdateMetadata) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	for name, uErr := range updates {
		if s, ok := pb.ldsWatchers[name]; ok {
			if uErr.Err != nil {
				// On error, keep previous version for each resource. But update
				// status and error.
				mdCopy := pb.ldsMD[name]
				mdCopy.ErrState = metadata.ErrState
				mdCopy.Status = metadata.Status
				pb.ldsMD[name] = mdCopy
				for wi := range s {
					wi.newError(uErr.Err)
				}
				continue
			}
			// If we get here, it means that the update is a valid one. Notify
			// watchers only if this is a first time update or it is different
			// from the one currently cached.
			if cur, ok := pb.ldsCache[name]; !ok || !proto.Equal(cur.Raw, uErr.Update.Raw) {
				for wi := range s {
					wi.newUpdate(uErr.Update)
				}
			}
			// Sync cache.
			pb.logger.Debugf("LDS resource with name %v, value %+v added to cache", name, pretty.ToJSON(uErr))
			pb.ldsCache[name] = uErr.Update
			// Set status to ACK, and clear error state. The metadata might be a
			// NACK metadata because some other resources in the same response
			// are invalid.
			mdCopy := metadata
			mdCopy.Status = xdsresource.ServiceStatusACKed
			mdCopy.ErrState = nil
			if metadata.ErrState != nil {
				mdCopy.Version = metadata.ErrState.Version
			}
			pb.ldsMD[name] = mdCopy
		}
	}
	// Resources not in the new update were removed by the server, so delete
	// them.
	for name := range pb.ldsCache {
		if _, ok := updates[name]; !ok {
			// If resource exists in cache, but not in the new update, delete
			// the resource from cache, and also send an resource not found
			// error to indicate resource removed.
			delete(pb.ldsCache, name)
			pb.ldsMD[name] = xdsresource.UpdateMetadata{Status: xdsresource.ServiceStatusNotExist}
			for wi := range pb.ldsWatchers[name] {
				wi.resourceNotFound()
			}
		}
	}
	// When LDS resource is removed, we don't delete corresponding RDS cached
	// data. The RDS watch will be canceled, and cache entry is removed when the
	// last watch is canceled.
}

// NewRouteConfigs is called when there's a new RDS update.
func (pb *Pubsub) NewRouteConfigs(updates map[string]xdsresource.RouteConfigUpdateErrTuple, metadata xdsresource.UpdateMetadata) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	// If no error received, the status is ACK.
	for name, uErr := range updates {
		if s, ok := pb.rdsWatchers[name]; ok {
			if uErr.Err != nil {
				// On error, keep previous version for each resource. But update
				// status and error.
				mdCopy := pb.rdsMD[name]
				mdCopy.ErrState = metadata.ErrState
				mdCopy.Status = metadata.Status
				pb.rdsMD[name] = mdCopy
				for wi := range s {
					wi.newError(uErr.Err)
				}
				continue
			}
			// If we get here, it means that the update is a valid one. Notify
			// watchers only if this is a first time update or it is different
			// from the one currently cached.
			if cur, ok := pb.rdsCache[name]; !ok || !proto.Equal(cur.Raw, uErr.Update.Raw) {
				for wi := range s {
					wi.newUpdate(uErr.Update)
				}
			}
			// Sync cache.
			pb.logger.Debugf("RDS resource with name %v, value %+v added to cache", name, pretty.ToJSON(uErr))
			pb.rdsCache[name] = uErr.Update
			// Set status to ACK, and clear error state. The metadata might be a
			// NACK metadata because some other resources in the same response
			// are invalid.
			mdCopy := metadata
			mdCopy.Status = xdsresource.ServiceStatusACKed
			mdCopy.ErrState = nil
			if metadata.ErrState != nil {
				mdCopy.Version = metadata.ErrState.Version
			}
			pb.rdsMD[name] = mdCopy
		}
	}
}

// NewClusters is called when there's a new CDS update.
func (pb *Pubsub) NewClusters(updates map[string]xdsresource.ClusterUpdateErrTuple, metadata xdsresource.UpdateMetadata) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	for name, uErr := range updates {
		if s, ok := pb.cdsWatchers[name]; ok {
			if uErr.Err != nil {
				// On error, keep previous version for each resource. But update
				// status and error.
				mdCopy := pb.cdsMD[name]
				mdCopy.ErrState = metadata.ErrState
				mdCopy.Status = metadata.Status
				pb.cdsMD[name] = mdCopy
				for wi := range s {
					// Send the watcher the individual error, instead of the
					// overall combined error from the metadata.ErrState.
					wi.newError(uErr.Err)
				}
				continue
			}
			// If we get here, it means that the update is a valid one. Notify
			// watchers only if this is a first time update or it is different
			// from the one currently cached.
			if cur, ok := pb.cdsCache[name]; !ok || !proto.Equal(cur.Raw, uErr.Update.Raw) {
				for wi := range s {
					wi.newUpdate(uErr.Update)
				}
			}
			// Sync cache.
			pb.logger.Debugf("CDS resource with name %v, value %+v added to cache", name, pretty.ToJSON(uErr))
			pb.cdsCache[name] = uErr.Update
			// Set status to ACK, and clear error state. The metadata might be a
			// NACK metadata because some other resources in the same response
			// are invalid.
			mdCopy := metadata
			mdCopy.Status = xdsresource.ServiceStatusACKed
			mdCopy.ErrState = nil
			if metadata.ErrState != nil {
				mdCopy.Version = metadata.ErrState.Version
			}
			pb.cdsMD[name] = mdCopy
		}
	}
	// Resources not in the new update were removed by the server, so delete
	// them.
	for name := range pb.cdsCache {
		if _, ok := updates[name]; !ok {
			// If resource exists in cache, but not in the new update, delete it
			// from cache, and also send an resource not found error to indicate
			// resource removed.
			delete(pb.cdsCache, name)
			pb.ldsMD[name] = xdsresource.UpdateMetadata{Status: xdsresource.ServiceStatusNotExist}
			for wi := range pb.cdsWatchers[name] {
				wi.resourceNotFound()
			}
		}
	}
	// When CDS resource is removed, we don't delete corresponding EDS cached
	// data. The EDS watch will be canceled, and cache entry is removed when the
	// last watch is canceled.
}

// NewEndpoints is called when there's anew EDS update.
func (pb *Pubsub) NewEndpoints(updates map[string]xdsresource.EndpointsUpdateErrTuple, metadata xdsresource.UpdateMetadata) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	for name, uErr := range updates {
		if s, ok := pb.edsWatchers[name]; ok {
			if uErr.Err != nil {
				// On error, keep previous version for each resource. But update
				// status and error.
				mdCopy := pb.edsMD[name]
				mdCopy.ErrState = metadata.ErrState
				mdCopy.Status = metadata.Status
				pb.edsMD[name] = mdCopy
				for wi := range s {
					// Send the watcher the individual error, instead of the
					// overall combined error from the metadata.ErrState.
					wi.newError(uErr.Err)
				}
				continue
			}
			// If we get here, it means that the update is a valid one. Notify
			// watchers only if this is a first time update or it is different
			// from the one currently cached.
			if cur, ok := pb.edsCache[name]; !ok || !proto.Equal(cur.Raw, uErr.Update.Raw) {
				for wi := range s {
					wi.newUpdate(uErr.Update)
				}
			}
			// Sync cache.
			pb.logger.Debugf("EDS resource with name %v, value %+v added to cache", name, pretty.ToJSON(uErr))
			pb.edsCache[name] = uErr.Update
			// Set status to ACK, and clear error state. The metadata might be a
			// NACK metadata because some other resources in the same response
			// are invalid.
			mdCopy := metadata
			mdCopy.Status = xdsresource.ServiceStatusACKed
			mdCopy.ErrState = nil
			if metadata.ErrState != nil {
				mdCopy.Version = metadata.ErrState.Version
			}
			pb.edsMD[name] = mdCopy
		}
	}
}

// NewConnectionError is called by the underlying xdsAPIClient when it receives
// a connection error. The error will be forwarded to all the resource watchers.
func (pb *Pubsub) NewConnectionError(err error) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	for _, s := range pb.ldsWatchers {
		for wi := range s {
			wi.newError(xdsresource.NewErrorf(xdsresource.ErrorTypeConnection, "xds: error received from xDS stream: %v", err))
		}
	}
	for _, s := range pb.rdsWatchers {
		for wi := range s {
			wi.newError(xdsresource.NewErrorf(xdsresource.ErrorTypeConnection, "xds: error received from xDS stream: %v", err))
		}
	}
	for _, s := range pb.cdsWatchers {
		for wi := range s {
			wi.newError(xdsresource.NewErrorf(xdsresource.ErrorTypeConnection, "xds: error received from xDS stream: %v", err))
		}
	}
	for _, s := range pb.edsWatchers {
		for wi := range s {
			wi.newError(xdsresource.NewErrorf(xdsresource.ErrorTypeConnection, "xds: error received from xDS stream: %v", err))
		}
	}
}
