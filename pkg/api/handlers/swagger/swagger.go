package swagger

import (
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/libpod/image"
	"github.com/containers/podman/v2/pkg/api/handlers"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/inspect"
	"github.com/docker/docker/api/types"
)

// History response
// swagger:response DocsHistory
type swagHistory struct {
	// in:body
	Body struct {
		handlers.HistoryResponse
	}
}

// Inspect response
// swagger:response DocsImageInspect
type swagImageInspect struct {
	// in:body
	Body struct {
		handlers.ImageInspect
	}
}

// Load response
// swagger:response DocsLibpodImagesLoadResponse
type swagLibpodImagesLoadResponse struct {
	// in:body
	Body entities.ImageLoadReport
}

// Import response
// swagger:response DocsLibpodImagesImportResponse
type swagLibpodImagesImportResponse struct {
	// in:body
	Body entities.ImageImportReport
}

// Pull response
// swagger:response DocsLibpodImagesPullResponse
type swagLibpodImagesPullResponse struct {
	// in:body
	Body handlers.LibpodImagesPullReport
}

// Remove response
// swagger:response DocsLibpodImagesRemoveResponse
type swagLibpodImagesRemoveResponse struct {
	// in:body
	Body handlers.LibpodImagesRemoveReport
}

// PlayKube response
// swagger:response DocsLibpodPlayKubeResponse
type swagLibpodPlayKubeResponse struct {
	// in:body
	Body entities.PlayKubeReport
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
	Body []handlers.ContainersPruneReport
}

// Prune containers
// swagger:response DocsLibpodPruneResponse
type swagLibpodContainerPruneReport struct {
	// in: body
	Body []handlers.LibpodContainersPruneReport
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
// swagger:response DocsContainerTopResponse
type swagContainerTopResponse struct {
	// in:body
	Body struct {
		handlers.ContainerTopOKBody
	}
}

// List processes in pod
// swagger:response DocsPodTopResponse
type swagPodTopResponse struct {
	// in:body
	Body struct {
		handlers.PodTopOKBody
	}
}

// List processes in pod
// swagger:response DocsPodStatsResponse
type swagPodStatsResponse struct {
	// in:body
	Body []*entities.PodStatsReport
}

// Inspect container
// swagger:response LibpodInspectContainerResponse
type swagLibpodInspectContainerResponse struct {
	// in:body
	Body struct {
		define.InspectContainerData
	}
}

// List pods
// swagger:response ListPodsResponse
type swagListPodsResponse struct {
	// in:body
	Body []entities.ListPodsReport
}

// Inspect pod
// swagger:response InspectPodResponse
type swagInspectPodResponse struct {
	// in:body
	Body struct {
		define.InspectPodData
	}
}

// Inspect volume
// swagger:response InspectVolumeResponse
type swagInspectVolumeResponse struct {
	// in:body
	Body struct {
		define.InspectVolumeData
	}
}

// Image tree response
// swagger:response LibpodImageTreeResponse
type swagImageTreeResponse struct {
	// in:body
	Body struct {
		handlers.ImageTreeResponse
	}
}
