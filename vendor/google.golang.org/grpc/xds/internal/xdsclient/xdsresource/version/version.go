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

// Package version defines constants to distinguish between supported xDS API
// versions.
package version

// TransportAPI refers to the API version for xDS transport protocol. This
// describes the xDS gRPC endpoint and version of DiscoveryRequest/Response used
// on the wire.
type TransportAPI int

const (
	// TransportV2 refers to the v2 xDS transport protocol.
	TransportV2 TransportAPI = iota
	// TransportV3 refers to the v3 xDS transport protocol.
	TransportV3
)

// Resource URLs. We need to be able to accept either version of the resource
// regardless of the version of the transport protocol in use.
const (
	googleapiPrefix = "type.googleapis.com/"

	V2ListenerType    = "envoy.api.v2.Listener"
	V2RouteConfigType = "envoy.api.v2.RouteConfiguration"
	V2ClusterType     = "envoy.api.v2.Cluster"
	V2EndpointsType   = "envoy.api.v2.ClusterLoadAssignment"

	V2ListenerURL        = googleapiPrefix + V2ListenerType
	V2RouteConfigURL     = googleapiPrefix + V2RouteConfigType
	V2ClusterURL         = googleapiPrefix + V2ClusterType
	V2EndpointsURL       = googleapiPrefix + V2EndpointsType
	V2HTTPConnManagerURL = googleapiPrefix + "envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager"

	V3ListenerType    = "envoy.config.listener.v3.Listener"
	V3RouteConfigType = "envoy.config.route.v3.RouteConfiguration"
	V3ClusterType     = "envoy.config.cluster.v3.Cluster"
	V3EndpointsType   = "envoy.config.endpoint.v3.ClusterLoadAssignment"

	V3ListenerURL             = googleapiPrefix + V3ListenerType
	V3RouteConfigURL          = googleapiPrefix + V3RouteConfigType
	V3ClusterURL              = googleapiPrefix + V3ClusterType
	V3EndpointsURL            = googleapiPrefix + V3EndpointsType
	V3HTTPConnManagerURL      = googleapiPrefix + "envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager"
	V3UpstreamTLSContextURL   = googleapiPrefix + "envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext"
	V3DownstreamTLSContextURL = googleapiPrefix + "envoy.extensions.transport_sockets.tls.v3.DownstreamTlsContext"
)
