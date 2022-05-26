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

// Package version defines APIs to deal with different versions of xDS.
package version

import (
	"context"
	"time"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/internal/grpclog"
	"google.golang.org/grpc/xds/internal/xdsclient/load"
	"google.golang.org/grpc/xds/internal/xdsclient/xdsresource"
	"google.golang.org/grpc/xds/internal/xdsclient/xdsresource/version"
	"google.golang.org/protobuf/types/known/anypb"
)

var (
	m = make(map[version.TransportAPI]func(opts BuildOptions) (VersionedClient, error))
)

// RegisterAPIClientBuilder registers a client builder for xDS transport protocol
// version specified by b.Version().
//
// NOTE: this function must only be called during initialization time (i.e. in
// an init() function), and is not thread-safe. If multiple builders are
// registered for the same version, the one registered last will take effect.
func RegisterAPIClientBuilder(v version.TransportAPI, f func(opts BuildOptions) (VersionedClient, error)) {
	m[v] = f
}

// GetAPIClientBuilder returns the client builder registered for the provided
// xDS transport API version.
func GetAPIClientBuilder(version version.TransportAPI) func(opts BuildOptions) (VersionedClient, error) {
	if f, ok := m[version]; ok {
		return f
	}
	return nil
}

// BuildOptions contains options to be passed to client builders.
type BuildOptions struct {
	// NodeProto contains the Node proto to be used in xDS requests. The actual
	// type depends on the transport protocol version used.
	NodeProto proto.Message
	// // Backoff returns the amount of time to backoff before retrying broken
	// // streams.
	// Backoff func(int) time.Duration
	// Logger provides enhanced logging capabilities.
	Logger *grpclog.PrefixLogger
}

// LoadReportingOptions contains configuration knobs for reporting load data.
type LoadReportingOptions struct {
	LoadStore *load.Store
}

// ErrResourceTypeUnsupported is an error used to indicate an unsupported xDS
// resource type. The wrapped ErrStr contains the details.
type ErrResourceTypeUnsupported struct {
	ErrStr string
}

// Error helps implements the error interface.
func (e ErrResourceTypeUnsupported) Error() string {
	return e.ErrStr
}

// VersionedClient is the interface to version specific operations of the
// client.
//
// It mainly deals with the type assertion from proto.Message to the real v2/v3
// types, and grpc.Stream to the versioned stream types.
type VersionedClient interface {
	// NewStream returns a new xDS client stream specific to the underlying
	// transport protocol version.
	NewStream(ctx context.Context, cc *grpc.ClientConn) (grpc.ClientStream, error)
	// SendRequest constructs and sends out a DiscoveryRequest message specific
	// to the underlying transport protocol version.
	SendRequest(s grpc.ClientStream, resourceNames []string, rType xdsresource.ResourceType, version, nonce, errMsg string) error
	// RecvResponse uses the provided stream to receive a response specific to
	// the underlying transport protocol version.
	RecvResponse(s grpc.ClientStream) (proto.Message, error)
	// ParseResponse type asserts message to the versioned response, and
	// retrieves the fields.
	ParseResponse(r proto.Message) (xdsresource.ResourceType, []*anypb.Any, string, string, error)

	// The following are LRS methods.

	// NewLoadStatsStream returns a new LRS client stream specific to the
	// underlying transport protocol version.
	NewLoadStatsStream(ctx context.Context, cc *grpc.ClientConn) (grpc.ClientStream, error)
	// SendFirstLoadStatsRequest constructs and sends the first request on the
	// LRS stream.
	SendFirstLoadStatsRequest(s grpc.ClientStream) error
	// HandleLoadStatsResponse receives the first response from the server which
	// contains the load reporting interval and the clusters for which the
	// server asks the client to report load for.
	//
	// If the response sets SendAllClusters to true, the returned clusters is
	// nil.
	HandleLoadStatsResponse(s grpc.ClientStream) (clusters []string, _ time.Duration, _ error)
	// SendLoadStatsRequest will be invoked at regular intervals to send load
	// report with load data reported since the last time this method was
	// invoked.
	SendLoadStatsRequest(s grpc.ClientStream, loads []*load.Data) error
}
