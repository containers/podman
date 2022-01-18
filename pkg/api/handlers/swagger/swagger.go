package swagger

import (
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/api/handlers"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/inspect"
	"github.com/docker/docker/api/types"
)

// Tree response
// swagger:response TreeResponse
type swagTree struct {
	// in:body
	Body struct {
		entities.ImageTreeReport
	}
}

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
	Body []struct {
		Untagged []string `json:"untagged"`
		Deleted  string   `json:"deleted"`
	}
}

// Search results
// swagger:response DocsSearchResponse
type swagSearchResponse struct {
	// in:body
	Body struct {
		// Index is the image index (e.g., "docker.io" or "quay.io")
		Index string
		// Name is the canonical name of the image (e.g., "docker.io/library/alpine").
		Name string
		// Description of the image.
		Description string
		// Stars is the number of stars of the image.
		Stars int
		// Official indicates if it's an official image.
		Official string
		// Automated indicates if the image was created by an automated build.
		Automated string
		// Tag is the image tag
		Tag string
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

// Rm containers
// swagger:response DocsLibpodContainerRmReport
type swagLibpodContainerRmReport struct {
	// in: body
	Body []handlers.LibpodContainersRmReport
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

// Get stats for one or more containers
// swagger:response ContainerStats
type swagContainerStatsResponse struct {
	// in:body
	Body struct {
		define.ContainerStats
	}
}
