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

// Package csds implements features to dump the status (xDS responses) the
// xds_client is using.
//
// Notice: This package is EXPERIMENTAL and may be changed or removed in a later
// release.
package csds

import (
	"context"
	"io"

	v3adminpb "github.com/envoyproxy/go-control-plane/envoy/admin/v3"
	v2corepb "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	v3corepb "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	v3statusgrpc "github.com/envoyproxy/go-control-plane/envoy/service/status/v3"
	v3statuspb "github.com/envoyproxy/go-control-plane/envoy/service/status/v3"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/xds/internal/xdsclient"
	"google.golang.org/grpc/xds/internal/xdsclient/xdsresource"
	"google.golang.org/protobuf/types/known/timestamppb"

	_ "google.golang.org/grpc/xds/internal/xdsclient/controller/version/v2" // Register v2 xds_client.
	_ "google.golang.org/grpc/xds/internal/xdsclient/controller/version/v3" // Register v3 xds_client.
)

var (
	logger       = grpclog.Component("xds")
	newXDSClient = func() xdsclient.XDSClient {
		c, err := xdsclient.New()
		if err != nil {
			logger.Warningf("failed to create xds client: %v", err)
			return nil
		}
		return c
	}
)

const (
	listenerTypeURL    = "envoy.config.listener.v3.Listener"
	routeConfigTypeURL = "envoy.config.route.v3.RouteConfiguration"
	clusterTypeURL     = "envoy.config.cluster.v3.Cluster"
	endpointsTypeURL   = "envoy.config.endpoint.v3.ClusterLoadAssignment"
)

// ClientStatusDiscoveryServer implementations interface ClientStatusDiscoveryServiceServer.
type ClientStatusDiscoveryServer struct {
	// xdsClient will always be the same in practice. But we keep a copy in each
	// server instance for testing.
	xdsClient xdsclient.XDSClient
}

// NewClientStatusDiscoveryServer returns an implementation of the CSDS server that can be
// registered on a gRPC server.
func NewClientStatusDiscoveryServer() (*ClientStatusDiscoveryServer, error) {
	return &ClientStatusDiscoveryServer{xdsClient: newXDSClient()}, nil
}

// StreamClientStatus implementations interface ClientStatusDiscoveryServiceServer.
func (s *ClientStatusDiscoveryServer) StreamClientStatus(stream v3statusgrpc.ClientStatusDiscoveryService_StreamClientStatusServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		resp, err := s.buildClientStatusRespForReq(req)
		if err != nil {
			return err
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}

// FetchClientStatus implementations interface ClientStatusDiscoveryServiceServer.
func (s *ClientStatusDiscoveryServer) FetchClientStatus(_ context.Context, req *v3statuspb.ClientStatusRequest) (*v3statuspb.ClientStatusResponse, error) {
	return s.buildClientStatusRespForReq(req)
}

// buildClientStatusRespForReq fetches the status from the client, and returns
// the response to be sent back to xdsclient.
//
// If it returns an error, the error is a status error.
func (s *ClientStatusDiscoveryServer) buildClientStatusRespForReq(req *v3statuspb.ClientStatusRequest) (*v3statuspb.ClientStatusResponse, error) {
	if s.xdsClient == nil {
		return &v3statuspb.ClientStatusResponse{}, nil
	}
	// Field NodeMatchers is unsupported, by design
	// https://github.com/grpc/proposal/blob/master/A40-csds-support.md#detail-node-matching.
	if len(req.NodeMatchers) != 0 {
		return nil, status.Errorf(codes.InvalidArgument, "node_matchers are not supported, request contains node_matchers: %v", req.NodeMatchers)
	}

	lds := dumpToGenericXdsConfig(listenerTypeURL, s.xdsClient.DumpLDS)
	rds := dumpToGenericXdsConfig(routeConfigTypeURL, s.xdsClient.DumpRDS)
	cds := dumpToGenericXdsConfig(clusterTypeURL, s.xdsClient.DumpCDS)
	eds := dumpToGenericXdsConfig(endpointsTypeURL, s.xdsClient.DumpEDS)
	configs := make([]*v3statuspb.ClientConfig_GenericXdsConfig, 0, len(lds)+len(rds)+len(cds)+len(eds))
	configs = append(configs, lds...)
	configs = append(configs, rds...)
	configs = append(configs, cds...)
	configs = append(configs, eds...)

	ret := &v3statuspb.ClientStatusResponse{
		Config: []*v3statuspb.ClientConfig{
			{
				Node:              nodeProtoToV3(s.xdsClient.BootstrapConfig().XDSServer.NodeProto),
				GenericXdsConfigs: configs,
			},
		},
	}
	return ret, nil
}

// Close cleans up the resources.
func (s *ClientStatusDiscoveryServer) Close() {
	if s.xdsClient != nil {
		s.xdsClient.Close()
	}
}

// nodeProtoToV3 converts the given proto into a v3.Node. n is from bootstrap
// config, it can be either v2.Node or v3.Node.
//
// If n is already a v3.Node, return it.
// If n is v2.Node, marshal and unmarshal it to v3.
// Otherwise, return nil.
//
// The default case (not v2 or v3) is nil, instead of error, because the
// resources in the response are more important than the node. The worst case is
// that the user will receive no Node info, but will still get resources.
func nodeProtoToV3(n proto.Message) *v3corepb.Node {
	var node *v3corepb.Node
	switch nn := n.(type) {
	case *v3corepb.Node:
		node = nn
	case *v2corepb.Node:
		v2, err := proto.Marshal(nn)
		if err != nil {
			logger.Warningf("Failed to marshal node (%v): %v", n, err)
			break
		}
		node = new(v3corepb.Node)
		if err := proto.Unmarshal(v2, node); err != nil {
			logger.Warningf("Failed to unmarshal node (%v): %v", v2, err)
		}
	default:
		logger.Warningf("node from bootstrap is %#v, only v2.Node and v3.Node are supported", nn)
	}
	return node
}

func dumpToGenericXdsConfig(typeURL string, dumpF func() map[string]xdsresource.UpdateWithMD) []*v3statuspb.ClientConfig_GenericXdsConfig {
	dump := dumpF()
	ret := make([]*v3statuspb.ClientConfig_GenericXdsConfig, 0, len(dump))
	for name, d := range dump {
		config := &v3statuspb.ClientConfig_GenericXdsConfig{
			TypeUrl:      typeURL,
			Name:         name,
			VersionInfo:  d.MD.Version,
			XdsConfig:    d.Raw,
			LastUpdated:  timestamppb.New(d.MD.Timestamp),
			ClientStatus: serviceStatusToProto(d.MD.Status),
		}
		if errState := d.MD.ErrState; errState != nil {
			config.ErrorState = &v3adminpb.UpdateFailureState{
				LastUpdateAttempt: timestamppb.New(errState.Timestamp),
				Details:           errState.Err.Error(),
				VersionInfo:       errState.Version,
			}
		}
		ret = append(ret, config)
	}
	return ret
}

func serviceStatusToProto(serviceStatus xdsresource.ServiceStatus) v3adminpb.ClientResourceStatus {
	switch serviceStatus {
	case xdsresource.ServiceStatusUnknown:
		return v3adminpb.ClientResourceStatus_UNKNOWN
	case xdsresource.ServiceStatusRequested:
		return v3adminpb.ClientResourceStatus_REQUESTED
	case xdsresource.ServiceStatusNotExist:
		return v3adminpb.ClientResourceStatus_DOES_NOT_EXIST
	case xdsresource.ServiceStatusACKed:
		return v3adminpb.ClientResourceStatus_ACKED
	case xdsresource.ServiceStatusNACKed:
		return v3adminpb.ClientResourceStatus_NACKED
	default:
		return v3adminpb.ClientResourceStatus_UNKNOWN
	}
}
