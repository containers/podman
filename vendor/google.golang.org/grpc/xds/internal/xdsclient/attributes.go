/*
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

package xdsclient

import (
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/xds/internal/xdsclient/bootstrap"
	"google.golang.org/grpc/xds/internal/xdsclient/load"
	"google.golang.org/grpc/xds/internal/xdsclient/xdsresource"
)

type clientKeyType string

const clientKey = clientKeyType("grpc.xds.internal.client.Client")

// XDSClient is a full fledged gRPC client which queries a set of discovery APIs
// (collectively termed as xDS) on a remote management server, to discover
// various dynamic resources.
type XDSClient interface {
	WatchListener(string, func(xdsresource.ListenerUpdate, error)) func()
	WatchRouteConfig(string, func(xdsresource.RouteConfigUpdate, error)) func()
	WatchCluster(string, func(xdsresource.ClusterUpdate, error)) func()
	WatchEndpoints(clusterName string, edsCb func(xdsresource.EndpointsUpdate, error)) (cancel func())
	ReportLoad(server string) (*load.Store, func())

	DumpLDS() map[string]xdsresource.UpdateWithMD
	DumpRDS() map[string]xdsresource.UpdateWithMD
	DumpCDS() map[string]xdsresource.UpdateWithMD
	DumpEDS() map[string]xdsresource.UpdateWithMD

	BootstrapConfig() *bootstrap.Config
	Close()
}

// FromResolverState returns the Client from state, or nil if not present.
func FromResolverState(state resolver.State) XDSClient {
	cs, _ := state.Attributes.Value(clientKey).(XDSClient)
	return cs
}

// SetClient sets c in state and returns the new state.
func SetClient(state resolver.State, c XDSClient) resolver.State {
	state.Attributes = state.Attributes.WithValue(clientKey, c)
	return state
}
