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

package v2

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"google.golang.org/grpc/internal/pretty"
	"google.golang.org/grpc/xds/internal/xdsclient/load"

	v2corepb "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	v2endpointpb "github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	lrsgrpc "github.com/envoyproxy/go-control-plane/envoy/service/load_stats/v2"
	lrspb "github.com/envoyproxy/go-control-plane/envoy/service/load_stats/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/xds/internal"
)

const clientFeatureLRSSendAllClusters = "envoy.lrs.supports_send_all_clusters"

type lrsStream lrsgrpc.LoadReportingService_StreamLoadStatsClient

func (v2c *client) NewLoadStatsStream(ctx context.Context, cc *grpc.ClientConn) (grpc.ClientStream, error) {
	c := lrsgrpc.NewLoadReportingServiceClient(cc)
	return c.StreamLoadStats(ctx)
}

func (v2c *client) SendFirstLoadStatsRequest(s grpc.ClientStream) error {
	stream, ok := s.(lrsStream)
	if !ok {
		return fmt.Errorf("lrs: Attempt to send request on unsupported stream type: %T", s)
	}
	node := proto.Clone(v2c.nodeProto).(*v2corepb.Node)
	if node == nil {
		node = &v2corepb.Node{}
	}
	node.ClientFeatures = append(node.ClientFeatures, clientFeatureLRSSendAllClusters)

	req := &lrspb.LoadStatsRequest{Node: node}
	v2c.logger.Infof("lrs: sending init LoadStatsRequest: %v", pretty.ToJSON(req))
	return stream.Send(req)
}

func (v2c *client) HandleLoadStatsResponse(s grpc.ClientStream) ([]string, time.Duration, error) {
	stream, ok := s.(lrsStream)
	if !ok {
		return nil, 0, fmt.Errorf("lrs: Attempt to receive response on unsupported stream type: %T", s)
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, 0, fmt.Errorf("lrs: failed to receive first response: %v", err)
	}
	v2c.logger.Infof("lrs: received first LoadStatsResponse: %+v", pretty.ToJSON(resp))

	interval, err := ptypes.Duration(resp.GetLoadReportingInterval())
	if err != nil {
		return nil, 0, fmt.Errorf("lrs: failed to convert report interval: %v", err)
	}

	if resp.ReportEndpointGranularity {
		// TODO: fixme to support per endpoint loads.
		return nil, 0, errors.New("lrs: endpoint loads requested, but not supported by current implementation")
	}

	clusters := resp.Clusters
	if resp.SendAllClusters {
		// Return nil to send stats for all clusters.
		clusters = nil
	}

	return clusters, interval, nil
}

func (v2c *client) SendLoadStatsRequest(s grpc.ClientStream, loads []*load.Data) error {
	stream, ok := s.(lrsStream)
	if !ok {
		return fmt.Errorf("lrs: Attempt to send request on unsupported stream type: %T", s)
	}

	clusterStats := make([]*v2endpointpb.ClusterStats, 0, len(loads))
	for _, sd := range loads {
		droppedReqs := make([]*v2endpointpb.ClusterStats_DroppedRequests, 0, len(sd.Drops))
		for category, count := range sd.Drops {
			droppedReqs = append(droppedReqs, &v2endpointpb.ClusterStats_DroppedRequests{
				Category:     category,
				DroppedCount: count,
			})
		}
		localityStats := make([]*v2endpointpb.UpstreamLocalityStats, 0, len(sd.LocalityStats))
		for l, localityData := range sd.LocalityStats {
			lid, err := internal.LocalityIDFromString(l)
			if err != nil {
				return err
			}
			loadMetricStats := make([]*v2endpointpb.EndpointLoadMetricStats, 0, len(localityData.LoadStats))
			for name, loadData := range localityData.LoadStats {
				loadMetricStats = append(loadMetricStats, &v2endpointpb.EndpointLoadMetricStats{
					MetricName:                    name,
					NumRequestsFinishedWithMetric: loadData.Count,
					TotalMetricValue:              loadData.Sum,
				})
			}
			localityStats = append(localityStats, &v2endpointpb.UpstreamLocalityStats{
				Locality: &v2corepb.Locality{
					Region:  lid.Region,
					Zone:    lid.Zone,
					SubZone: lid.SubZone,
				},
				TotalSuccessfulRequests: localityData.RequestStats.Succeeded,
				TotalRequestsInProgress: localityData.RequestStats.InProgress,
				TotalErrorRequests:      localityData.RequestStats.Errored,
				LoadMetricStats:         loadMetricStats,
				UpstreamEndpointStats:   nil, // TODO: populate for per endpoint loads.
			})
		}

		clusterStats = append(clusterStats, &v2endpointpb.ClusterStats{
			ClusterName:           sd.Cluster,
			ClusterServiceName:    sd.Service,
			UpstreamLocalityStats: localityStats,
			TotalDroppedRequests:  sd.TotalDrops,
			DroppedRequests:       droppedReqs,
			LoadReportInterval:    ptypes.DurationProto(sd.ReportInterval),
		})

	}

	req := &lrspb.LoadStatsRequest{ClusterStats: clusterStats}
	v2c.logger.Infof("lrs: sending LRS loads: %+v", pretty.ToJSON(req))
	return stream.Send(req)
}
