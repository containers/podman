package handlers

import (
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/inspect"
	"github.com/docker/docker/api/types"
)

// History response
// swagger:response DocsHistory
type swagHistory struct {
	// in:body
	Body struct {
		HistoryResponse
	}
}

// Inspect response
// swagger:response DocsImageInspect
type swagImageInspect struct {
	// in:body
	Body struct {
		ImageInspect
	}
}

// Delete response
// swagger:response DocsImageDeleteResponse
type swagImageDeleteResponse struct {
	// in:body
	Body []image.ImageDeleteResponse
}

// Search results
// swagger:response DocsSearchResponse
type swagSearchResponse struct {
	// in:body
	Body struct {
		image.SearchResult
	}
}

// Inspect image
// swagger:response DocsLibpodInspectImageResponse
type swagLibpodInspectImageResponse struct {
	// in:body
	Body struct {
		inspect.ImageData
	}
}

// Prune containers
// swagger:response DocsContainerPruneReport
type swagContainerPruneReport struct {
	// in: body
	Body []ContainersPruneReport
}

// Inspect container
// swagger:response DocsContainerInspectResponse
type swagContainerInspectResponse struct {
	// in:body
	Body struct {
		types.ContainerJSON
	}
}

// List processes in container
// swagger:response DockerTopResponse
type swagDockerTopResponse struct {
	// in:body
	Body struct {
		ContainerTopOKBody
	}
}

// List containers
// swagger:response LibpodListContainersResponse
type swagLibpodListContainersResponse struct {
	// in:body
	Body []shared.PsContainerOutput
}

// Inspect container
// swagger:response LibpodInspectContainerResponse
type swagLibpodInspectContainerResponse struct {
	// in:body
	Body struct {
		libpod.InspectContainerData
	}
}

// List pods
// swagger:response ListPodsResponse
type swagListPodsResponse struct {
	// in:body
	Body struct {
		libpod.PodInspect
	}
}

// Inspect pod
// swagger:response InspectPodResponse
type swagInspectPodResponse struct {
	// in:body
	Body struct {
		libpod.PodInspect
	}
}

// Inspect volume
// swagger:response InspectVolumeResponse
type swagInspectVolumeResponse struct {
	// in:body
	Body struct {
		libpod.InspectVolumeData
	}
}
