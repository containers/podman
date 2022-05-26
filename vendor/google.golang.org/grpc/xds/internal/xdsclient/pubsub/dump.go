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
	anypb "github.com/golang/protobuf/ptypes/any"
	"google.golang.org/grpc/xds/internal/xdsclient/xdsresource"
)

func rawFromCache(s string, cache interface{}) *anypb.Any {
	switch c := cache.(type) {
	case map[string]xdsresource.ListenerUpdate:
		if v, ok := c[s]; ok {
			return v.Raw
		}
		return nil
	case map[string]xdsresource.RouteConfigUpdate:
		if v, ok := c[s]; ok {
			return v.Raw
		}
		return nil
	case map[string]xdsresource.ClusterUpdate:
		if v, ok := c[s]; ok {
			return v.Raw
		}
		return nil
	case map[string]xdsresource.EndpointsUpdate:
		if v, ok := c[s]; ok {
			return v.Raw
		}
		return nil
	default:
		return nil
	}
}

// Dump dumps the resource for the given type.
func (pb *Pubsub) Dump(t xdsresource.ResourceType) map[string]xdsresource.UpdateWithMD {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	var (
		md    map[string]xdsresource.UpdateMetadata
		cache interface{}
	)
	switch t {
	case xdsresource.ListenerResource:
		md = pb.ldsMD
		cache = pb.ldsCache
	case xdsresource.RouteConfigResource:
		md = pb.rdsMD
		cache = pb.rdsCache
	case xdsresource.ClusterResource:
		md = pb.cdsMD
		cache = pb.cdsCache
	case xdsresource.EndpointsResource:
		md = pb.edsMD
		cache = pb.edsCache
	default:
		pb.logger.Errorf("dumping resource of unknown type: %v", t)
		return nil
	}

	ret := make(map[string]xdsresource.UpdateWithMD, len(md))
	for s, md := range md {
		ret[s] = xdsresource.UpdateWithMD{
			MD:  md,
			Raw: rawFromCache(s, cache),
		}
	}
	return ret
}
