package compat

import (
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/docker/docker/api/types"
)

// Create container
// swagger:response ContainerCreateResponse
type swagCtrCreateResponse struct {
	// in:body
	Body struct {
		entities.ContainerCreateResponse
	}
}

// Wait container
// swagger:response ContainerWaitResponse
type swagCtrWaitResponse struct {
	// in:body
	Body struct {
		// container exit code
		StatusCode int
		Error      struct {
			Message string
		}
	}
}

// Network inspect
// swagger:response CompatNetworkInspect
type swagCompatNetworkInspect struct {
	// in:body
	Body types.NetworkResource
}

// Network list
// swagger:response CompatNetworkList
type swagCompatNetworkList struct {
	// in:body
	Body []types.NetworkResource
}

// Network create
// swagger:model NetworkCreateRequest
type NetworkCreateRequest struct {
	types.NetworkCreateRequest
}

// Network create
// swagger:response CompatNetworkCreate
type swagCompatNetworkCreateResponse struct {
	// in:body
	Body struct{ types.NetworkCreate }
}

// Network disconnect
// swagger:model NetworkCompatConnectRequest
type swagCompatNetworkConnectRequest struct {
	types.NetworkConnect
}

// Network disconnect
// swagger:model NetworkCompatDisconnectRequest
type swagCompatNetworkDisconnectRequest struct {
	types.NetworkDisconnect
}
