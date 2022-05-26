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

import "google.golang.org/grpc/xds/internal/xdsclient/xdsresource"

// UpdateHandler receives and processes (by taking appropriate actions) xDS
// resource updates from an APIClient for a specific version.
//
// It's a subset of the APIs of a *Pubsub.
type UpdateHandler interface {
	// NewListeners handles updates to xDS listener resources.
	NewListeners(map[string]xdsresource.ListenerUpdateErrTuple, xdsresource.UpdateMetadata)
	// NewRouteConfigs handles updates to xDS RouteConfiguration resources.
	NewRouteConfigs(map[string]xdsresource.RouteConfigUpdateErrTuple, xdsresource.UpdateMetadata)
	// NewClusters handles updates to xDS Cluster resources.
	NewClusters(map[string]xdsresource.ClusterUpdateErrTuple, xdsresource.UpdateMetadata)
	// NewEndpoints handles updates to xDS ClusterLoadAssignment (or tersely
	// referred to as Endpoints) resources.
	NewEndpoints(map[string]xdsresource.EndpointsUpdateErrTuple, xdsresource.UpdateMetadata)
	// NewConnectionError handles connection errors from the xDS stream. The
	// error will be reported to all the resource watchers.
	NewConnectionError(err error)
}
