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

package server

import (
	"sync"

	"google.golang.org/grpc/xds/internal/xdsclient/xdsresource"
)

// rdsHandlerUpdate wraps the full RouteConfigUpdate that are dynamically
// queried for a given server side listener.
type rdsHandlerUpdate struct {
	updates map[string]xdsresource.RouteConfigUpdate
	err     error
}

// rdsHandler handles any RDS queries that need to be started for a given server
// side listeners Filter Chains (i.e. not inline).
type rdsHandler struct {
	xdsC XDSClient

	mu      sync.Mutex
	updates map[string]xdsresource.RouteConfigUpdate
	cancels map[string]func()

	// For a rdsHandler update, the only update wrapped listener cares about is
	// most recent one, so this channel will be opportunistically drained before
	// sending any new updates.
	updateChannel chan rdsHandlerUpdate
}

// newRDSHandler creates a new rdsHandler to watch for RDS resources.
// listenerWrapper updates the list of route names to watch by calling
// updateRouteNamesToWatch() upon receipt of new Listener configuration.
func newRDSHandler(xdsC XDSClient, ch chan rdsHandlerUpdate) *rdsHandler {
	return &rdsHandler{
		xdsC:          xdsC,
		updateChannel: ch,
		updates:       make(map[string]xdsresource.RouteConfigUpdate),
		cancels:       make(map[string]func()),
	}
}

// updateRouteNamesToWatch handles a list of route names to watch for a given
// server side listener (if a filter chain specifies dynamic RDS configuration).
// This function handles all the logic with respect to any routes that may have
// been added or deleted as compared to what was previously present.
func (rh *rdsHandler) updateRouteNamesToWatch(routeNamesToWatch map[string]bool) {
	rh.mu.Lock()
	defer rh.mu.Unlock()
	// Add and start watches for any routes for any new routes in
	// routeNamesToWatch.
	for routeName := range routeNamesToWatch {
		if _, ok := rh.cancels[routeName]; !ok {
			func(routeName string) {
				rh.cancels[routeName] = rh.xdsC.WatchRouteConfig(routeName, func(update xdsresource.RouteConfigUpdate, err error) {
					rh.handleRouteUpdate(routeName, update, err)
				})
			}(routeName)
		}
	}

	// Delete and cancel watches for any routes from persisted routeNamesToWatch
	// that are no longer present.
	for routeName := range rh.cancels {
		if _, ok := routeNamesToWatch[routeName]; !ok {
			rh.cancels[routeName]()
			delete(rh.cancels, routeName)
			delete(rh.updates, routeName)
		}
	}

	// If the full list (determined by length) of updates are now successfully
	// updated, the listener is ready to be updated.
	if len(rh.updates) == len(rh.cancels) && len(routeNamesToWatch) != 0 {
		drainAndPush(rh.updateChannel, rdsHandlerUpdate{updates: rh.updates})
	}
}

// handleRouteUpdate persists the route config for a given route name, and also
// sends an update to the Listener Wrapper on an error received or if the rds
// handler has a full collection of updates.
func (rh *rdsHandler) handleRouteUpdate(routeName string, update xdsresource.RouteConfigUpdate, err error) {
	if err != nil {
		drainAndPush(rh.updateChannel, rdsHandlerUpdate{err: err})
		return
	}
	rh.mu.Lock()
	defer rh.mu.Unlock()
	rh.updates[routeName] = update

	// If the full list (determined by length) of updates have successfully
	// updated, the listener is ready to be updated.
	if len(rh.updates) == len(rh.cancels) {
		drainAndPush(rh.updateChannel, rdsHandlerUpdate{updates: rh.updates})
	}
}

func drainAndPush(ch chan rdsHandlerUpdate, update rdsHandlerUpdate) {
	select {
	case <-ch:
	default:
	}
	ch <- update
}

// close() is meant to be called by wrapped listener when the wrapped listener
// is closed, and it cleans up resources by canceling all the active RDS
// watches.
func (rh *rdsHandler) close() {
	rh.mu.Lock()
	defer rh.mu.Unlock()
	for _, cancel := range rh.cancels {
		cancel()
	}
}
