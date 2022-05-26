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

package xdsclient

import (
	"google.golang.org/grpc/xds/internal/xdsclient/xdsresource"
)

func mergeMaps(maps []map[string]xdsresource.UpdateWithMD) map[string]xdsresource.UpdateWithMD {
	ret := make(map[string]xdsresource.UpdateWithMD)
	for _, m := range maps {
		for k, v := range m {
			ret[k] = v
		}
	}
	return ret
}

func (c *clientImpl) dump(t xdsresource.ResourceType) map[string]xdsresource.UpdateWithMD {
	c.authorityMu.Lock()
	defer c.authorityMu.Unlock()
	maps := make([]map[string]xdsresource.UpdateWithMD, 0, len(c.authorities))
	for _, a := range c.authorities {
		maps = append(maps, a.dump(t))
	}
	return mergeMaps(maps)
}

// DumpLDS returns the status and contents of LDS.
func (c *clientImpl) DumpLDS() map[string]xdsresource.UpdateWithMD {
	return c.dump(xdsresource.ListenerResource)
}

// DumpRDS returns the status and contents of RDS.
func (c *clientImpl) DumpRDS() map[string]xdsresource.UpdateWithMD {
	return c.dump(xdsresource.RouteConfigResource)
}

// DumpCDS returns the status and contents of CDS.
func (c *clientImpl) DumpCDS() map[string]xdsresource.UpdateWithMD {
	return c.dump(xdsresource.ClusterResource)
}

// DumpEDS returns the status and contents of EDS.
func (c *clientImpl) DumpEDS() map[string]xdsresource.UpdateWithMD {
	return c.dump(xdsresource.EndpointsResource)
}
