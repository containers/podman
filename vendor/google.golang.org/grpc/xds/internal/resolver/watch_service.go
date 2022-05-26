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
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc/internal/grpclog"
	"google.golang.org/grpc/internal/pretty"
	"google.golang.org/grpc/xds/internal/clusterspecifier"
	"google.golang.org/grpc/xds/internal/xdsclient"
	"google.golang.org/grpc/xds/internal/xdsclient/xdsresource"
)

// serviceUpdate contains information received from the LDS/RDS responses which
// are of interest to the xds resolver. The RDS request is built by first
// making a LDS to get the RouteConfig name.
type serviceUpdate struct {
	// virtualHost contains routes and other configuration to route RPCs.
	virtualHost *xdsresource.VirtualHost
	// clusterSpecifierPlugins contains the configurations for any cluster
	// specifier plugins emitted by the xdsclient.
	clusterSpecifierPlugins map[string]clusterspecifier.BalancerConfig
	// ldsConfig contains configuration that applies to all routes.
	ldsConfig ldsConfig
}

// ldsConfig contains information received from the LDS responses which are of
// interest to the xds resolver.
type ldsConfig struct {
	// maxStreamDuration is from the HTTP connection manager's
	// common_http_protocol_options field.
	maxStreamDuration time.Duration
	httpFilterConfig  []xdsresource.HTTPFilter
}

// watchService uses LDS and RDS to discover information about the provided
// serviceName.
//
// Note that during race (e.g. an xDS response is received while the user is
// calling cancel()), there's a small window where the callback can be called
// after the watcher is canceled. The caller needs to handle this case.
func watchService(c xdsclient.XDSClient, serviceName string, cb func(serviceUpdate, error), logger *grpclog.PrefixLogger) (cancel func()) {
	w := &serviceUpdateWatcher{
		logger:      logger,
		c:           c,
		serviceName: serviceName,
		serviceCb:   cb,
	}
	w.ldsCancel = c.WatchListener(serviceName, w.handleLDSResp)

	return w.close
}

// serviceUpdateWatcher handles LDS and RDS response, and calls the service
// callback at the right time.
type serviceUpdateWatcher struct {
	logger      *grpclog.PrefixLogger
	c           xdsclient.XDSClient
	serviceName string
	ldsCancel   func()
	serviceCb   func(serviceUpdate, error)
	lastUpdate  serviceUpdate

	mu        sync.Mutex
	closed    bool
	rdsName   string
	rdsCancel func()
}

func (w *serviceUpdateWatcher) handleLDSResp(update xdsresource.ListenerUpdate, err error) {
	w.logger.Infof("received LDS update: %+v, err: %v", pretty.ToJSON(update), err)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	if err != nil {
		// We check the error type and do different things. For now, the only
		// type we check is ResourceNotFound, which indicates the LDS resource
		// was removed, and besides sending the error to callback, we also
		// cancel the RDS watch.
		if xdsresource.ErrType(err) == xdsresource.ErrorTypeResourceNotFound && w.rdsCancel != nil {
			w.rdsCancel()
			w.rdsName = ""
			w.rdsCancel = nil
			w.lastUpdate = serviceUpdate{}
		}
		// The other error cases still return early without canceling the
		// existing RDS watch.
		w.serviceCb(serviceUpdate{}, err)
		return
	}

	w.lastUpdate.ldsConfig = ldsConfig{
		maxStreamDuration: update.MaxStreamDuration,
		httpFilterConfig:  update.HTTPFilters,
	}

	if update.InlineRouteConfig != nil {
		// If there was an RDS watch, cancel it.
		w.rdsName = ""
		if w.rdsCancel != nil {
			w.rdsCancel()
			w.rdsCancel = nil
		}

		// Handle the inline RDS update as if it's from an RDS watch.
		w.applyRouteConfigUpdate(*update.InlineRouteConfig)
		return
	}

	// RDS name from update is not an empty string, need RDS to fetch the
	// routes.

	if w.rdsName == update.RouteConfigName {
		// If the new RouteConfigName is same as the previous, don't cancel and
		// restart the RDS watch.
		//
		// If the route name did change, then we must wait until the first RDS
		// update before reporting this LDS config.
		if w.lastUpdate.virtualHost != nil {
			// We want to send an update with the new fields from the new LDS
			// (e.g. max stream duration), and old fields from the the previous
			// RDS.
			//
			// But note that this should only happen when virtual host is set,
			// which means an RDS was received.
			w.serviceCb(w.lastUpdate, nil)
		}
		return
	}
	w.rdsName = update.RouteConfigName
	if w.rdsCancel != nil {
		w.rdsCancel()
	}
	w.rdsCancel = w.c.WatchRouteConfig(update.RouteConfigName, w.handleRDSResp)
}

func (w *serviceUpdateWatcher) applyRouteConfigUpdate(update xdsresource.RouteConfigUpdate) {
	matchVh := xdsresource.FindBestMatchingVirtualHost(w.serviceName, update.VirtualHosts)
	if matchVh == nil {
		// No matching virtual host found.
		w.serviceCb(serviceUpdate{}, fmt.Errorf("no matching virtual host found for %q", w.serviceName))
		return
	}

	w.lastUpdate.virtualHost = matchVh
	w.lastUpdate.clusterSpecifierPlugins = update.ClusterSpecifierPlugins
	w.serviceCb(w.lastUpdate, nil)
}

func (w *serviceUpdateWatcher) handleRDSResp(update xdsresource.RouteConfigUpdate, err error) {
	w.logger.Infof("received RDS update: %+v, err: %v", pretty.ToJSON(update), err)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	if w.rdsCancel == nil {
		// This mean only the RDS watch is canceled, can happen if the LDS
		// resource is removed.
		return
	}
	if err != nil {
		w.serviceCb(serviceUpdate{}, err)
		return
	}
	w.applyRouteConfigUpdate(update)
}

func (w *serviceUpdateWatcher) close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	w.ldsCancel()
	if w.rdsCancel != nil {
		w.rdsCancel()
		w.rdsCancel = nil
	}
}
