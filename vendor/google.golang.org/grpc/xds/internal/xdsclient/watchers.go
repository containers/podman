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
 */

package xdsclient

import (
	"google.golang.org/grpc/xds/internal/xdsclient/xdsresource"
)

// WatchListener uses LDS to discover information about the provided listener.
//
// Note that during race (e.g. an xDS response is received while the user is
// calling cancel()), there's a small window where the callback can be called
// after the watcher is canceled. The caller needs to handle this case.
func (c *clientImpl) WatchListener(serviceName string, cb func(xdsresource.ListenerUpdate, error)) (cancel func()) {
	n := xdsresource.ParseName(serviceName)
	a, unref, err := c.findAuthority(n)
	if err != nil {
		cb(xdsresource.ListenerUpdate{}, err)
		return func() {}
	}
	cancelF := a.watchListener(n.String(), cb)
	return func() {
		cancelF()
		unref()
	}
}

// WatchRouteConfig starts a listener watcher for the service.
//
// Note that during race (e.g. an xDS response is received while the user is
// calling cancel()), there's a small window where the callback can be called
// after the watcher is canceled. The caller needs to handle this case.
func (c *clientImpl) WatchRouteConfig(routeName string, cb func(xdsresource.RouteConfigUpdate, error)) (cancel func()) {
	n := xdsresource.ParseName(routeName)
	a, unref, err := c.findAuthority(n)
	if err != nil {
		cb(xdsresource.RouteConfigUpdate{}, err)
		return func() {}
	}
	cancelF := a.watchRouteConfig(n.String(), cb)
	return func() {
		cancelF()
		unref()
	}
}

// WatchCluster uses CDS to discover information about the provided
// clusterName.
//
// WatchCluster can be called multiple times, with same or different
// clusterNames. Each call will start an independent watcher for the resource.
//
// Note that during race (e.g. an xDS response is received while the user is
// calling cancel()), there's a small window where the callback can be called
// after the watcher is canceled. The caller needs to handle this case.
func (c *clientImpl) WatchCluster(clusterName string, cb func(xdsresource.ClusterUpdate, error)) (cancel func()) {
	n := xdsresource.ParseName(clusterName)
	a, unref, err := c.findAuthority(n)
	if err != nil {
		cb(xdsresource.ClusterUpdate{}, err)
		return func() {}
	}
	cancelF := a.watchCluster(n.String(), cb)
	return func() {
		cancelF()
		unref()
	}
}

// WatchEndpoints uses EDS to discover endpoints in the provided clusterName.
//
// WatchEndpoints can be called multiple times, with same or different
// clusterNames. Each call will start an independent watcher for the resource.
//
// Note that during race (e.g. an xDS response is received while the user is
// calling cancel()), there's a small window where the callback can be called
// after the watcher is canceled. The caller needs to handle this case.
func (c *clientImpl) WatchEndpoints(clusterName string, cb func(xdsresource.EndpointsUpdate, error)) (cancel func()) {
	n := xdsresource.ParseName(clusterName)
	a, unref, err := c.findAuthority(n)
	if err != nil {
		cb(xdsresource.EndpointsUpdate{}, err)
		return func() {}
	}
	cancelF := a.watchEndpoints(n.String(), cb)
	return func() {
		cancelF()
		unref()
	}
}
