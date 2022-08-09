//nolint:deadcode,unused // these types are used to wire generated swagger to API code
package swagger

import (
	"github.com/containers/common/libnetwork/types"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/api/handlers"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/entities/reports"
	"github.com/containers/podman/v4/pkg/inspect"
	dockerAPI "github.com/docker/docker/api/types"
	dockerVolume "github.com/docker/docker/api/types/volume"
)

// Image Tree
// swagger:response
type treeResponse struct {
	// in:body
	Body entities.ImageTreeReport
}

// Image History
// swagger:response
type history struct {
	// in:body
	Body handlers.HistoryResponse
}

// Image Inspect
// swagger:response
type imageInspect struct {
	// in:body
	Body handlers.ImageInspect
}

// Image Load
// swagger:response
type imagesLoadResponseLibpod struct {
	// in:body
	Body entities.ImageLoadReport
}

// Image Scp
// swagger:response
type imagesScpResponseLibpod struct {
	// in:body
	Body reports.ScpReport
}

// Image Import
// swagger:response
type imagesImportResponseLibpod struct {
	// in:body
	Body entities.ImageImportReport
}

// Image Pull
// swagger:response
type imagesPullResponseLibpod struct {
	// in:body
	Body handlers.LibpodImagesPullReport
}

// Image Remove
// swagger:response
type imagesRemoveResponseLibpod struct {
	// in:body
	Body handlers.LibpodImagesRemoveReport
}

// PlayKube response
// swagger:response
type playKubeResponseLibpod struct {
	// in:body
	Body entities.PlayKubeReport
}

// Image Delete
// swagger:response
type imageDeleteResponse struct {
	// in:body
	Body []struct {
		Untagged []string `json:"untagged"`
		Deleted  string   `json:"deleted"`
	}
}

// Registry Search
// swagger:response
type registrySearchResponse struct {
	// in:body
	Body struct {
		// Index is the image index
		// example: quay.io
		Index string
		// Name is the canonical name of the image
		// example: docker.io/library/alpine"
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

// Inspect Image
// swagger:response
type inspectImageResponseLibpod struct {
	// in:body
	Body inspect.ImageData
}

// Inspect container
// swagger:response
type containerInspectResponse struct {
	// in:body
	Body dockerAPI.ContainerJSON
}

// List processes in container
// swagger:response
type containerTopResponse struct {
	// in:body
	Body handlers.ContainerTopOKBody
}

// List processes in pod
// swagger:response
type podTopResponse struct {
	// in:body
	Body handlers.PodTopOKBody
}

// Pod Statistics
// swagger:response
type podStatsResponse struct {
	// in:body
	Body []entities.PodStatsReport
}

// Inspect container
// swagger:response
type containerInspectResponseLibpod struct {
	// in:body
	Body define.InspectContainerData
}

// List pods
// swagger:response
type podsListResponse struct {
	// in:body
	Body []entities.ListPodsReport
}

// Inspect pod
// swagger:response
type podInspectResponse struct {
	// in:body
	Body define.InspectPodData
}

// Volume details
// swagger:response
type volumeCreateResponse struct {
	// in:body
	Body entities.VolumeConfigResponse
}

// Healthcheck Results
// swagger:response
type healthCheck struct {
	// in:body
	Body define.HealthCheckResults
}

// Version
// swagger:response
type versionResponse struct {
	// in:body
	Body entities.ComponentVersion
}

// Disk usage
// swagger:response
type systemDiskUsage struct {
	// in:body
	Body entities.SystemDfReport
}

// System Prune results
// swagger:response
type systemPruneResponse struct {
	// in:body
	Body entities.SystemPruneReport
}

// Auth response
// swagger:response
type systemAuthResponse struct {
	// in:body
	Body entities.AuthReport
}

// Exec Session Inspect
// swagger:response
type execSessionInspect struct {
	// in:body
	Body define.InspectExecSession
}

// Image summary for compat API
// swagger:response
type imageList struct {
	// in:body
	Body []dockerAPI.ImageSummary
}

// Image summary for libpod API
// swagger:response
type imageListLibpod struct {
	// in:body
	Body []entities.ImageSummary
}

// List Containers
// swagger:response
type containersList struct {
	// in:body
	Body []handlers.Container
}

// This response definition is used for both the create and inspect endpoints
// swagger:response
type volumeInspect struct {
	// in:body
	Body dockerAPI.Volume
}

// Volume prune
// swagger:response
type volumePruneResponse struct {
	// in:body
	Body dockerAPI.VolumesPruneReport
}

// Volume List
// swagger:response
type volumeList struct {
	// in:body
	Body dockerVolume.VolumeListOKBody
}

// Volume list
// swagger:response
type volumeListLibpod struct {
	// in:body
	Body []entities.VolumeConfigResponse
}

// Image Prune
// swagger:response
type imagesPruneLibpod struct {
	// in:body
	Body []reports.PruneReport
}

// Remove Containers
// swagger:response
type containerRemoveLibpod struct {
	// in: body
	Body []handlers.LibpodContainersRmReport
}

// Prune Containers
// swagger:response
type containersPrune struct {
	// in: body
	Body []handlers.ContainersPruneReport
}

// Prune Containers
// swagger:response
type containersPruneLibpod struct {
	// in: body
	Body []handlers.ContainersPruneReportLibpod
}

// Get stats for one or more containers
// swagger:response
type containerStats struct {
	// in:body
	Body define.ContainerStats
}

// Volume Prune
// swagger:response
type volumePruneLibpod struct {
	// in:body
	Body []reports.PruneReport
}

// Create container
// swagger:response
type containerCreateResponse struct {
	// in:body
	Body entities.ContainerCreateResponse
}

type containerUpdateResponse struct {
	// in:body
	ID string
}

// Wait container
// swagger:response
type containerWaitResponse struct {
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
// swagger:response
type networkInspectCompat struct {
	// in:body
	Body dockerAPI.NetworkResource
}

// Network list
// swagger:response
type networkListCompat struct {
	// in:body
	Body []dockerAPI.NetworkResource
}

// List Containers
// swagger:response
type containersListLibpod struct {
	// in:body
	Body []entities.ListContainer
}

// Inspect Manifest
// swagger:response
type manifestInspect struct {
	// in:body
	Body manifest.Schema2List
}

// Kill Pod
// swagger:response
type podKillResponse struct {
	// in:body
	Body entities.PodKillReport
}

// Pause pod
// swagger:response
type podPauseResponse struct {
	// in:body
	Body entities.PodPauseReport
}

// Unpause pod
// swagger:response
type podUnpauseResponse struct {
	// in:body
	Body entities.PodUnpauseReport
}

// Stop pod
// swagger:response
type podStopResponse struct {
	// in:body
	Body entities.PodStopReport
}

// Restart pod
// swagger:response
type podRestartResponse struct {
	// in:body
	Body entities.PodRestartReport
}

// Start pod
// swagger:response
type podStartResponse struct {
	// in:body
	Body entities.PodStartReport
}

// Prune pod
// swagger:response
type podPruneResponse struct {
	// in:body
	Body entities.PodPruneReport
}

// Rm pod
// swagger:response
type podRmResponse struct {
	// in:body
	Body entities.PodRmReport
}

// Info
// swagger:response
type infoResponse struct {
	// in:body
	Body define.Info
}

// Network Delete
// swagger:response
type networkRmResponse struct {
	// in:body
	Body []entities.NetworkRmReport
}

// Network inspect
// swagger:response
type networkInspectResponse struct {
	// in:body
	Body types.Network
}

// Network list
// swagger:response
type networkListLibpod struct {
	// in:body
	Body []types.Network
}

// Network create
// swagger:model
type networkCreateLibpod struct {
	// in:body
	types.Network
}

// Network create
// swagger:response
type networkCreateResponse struct {
	// in:body
	Body types.Network
}

// Network prune
// swagger:response
type networkPruneResponse struct {
	// in:body
	Body []entities.NetworkPruneReport
}
