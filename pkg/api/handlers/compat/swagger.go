package compat

import (
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/storage/pkg/archive"
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

// Object Changes
// swagger:response Changes
type swagChangesResponse struct {
	// in:body
	Body struct {
		Changes []archive.Change
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
