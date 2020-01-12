package libpod

import "github.com/containernetworking/cni/libcni"

// List networks
// swagger:response ListNetworksResponse
type swagListNetworks struct {
	// in:body
	Body struct {
		libcni.NetworkConfigList
	}
}

// List networks
// swagger:response InspectNetworkResponse
type swagInspectNetworkResponse map[string]interface{}
