package server

import (
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/domain/entities"
)

// No such image
// swagger:response NoSuchImage
type swagErrNoSuchImage struct {
	// in:body
	Body struct {
		entities.ErrorModel
	}
}

// No such container
// swagger:response NoSuchContainer
type swagErrNoSuchContainer struct {
	// in:body
	Body struct {
		entities.ErrorModel
	}
}

// No such network
// swagger:response NoSuchNetwork
type swagErrNoSuchNetwork struct {
	// in:body
	Body struct {
		entities.ErrorModel
	}
}

// No such exec instance
// swagger:response NoSuchExecInstance
type swagErrNoSuchExecInstance struct {
	// in:body
	Body struct {
		entities.ErrorModel
	}
}

// No such volume
// swagger:response NoSuchVolume
type swagErrNoSuchVolume struct {
	// in:body
	Body struct {
		entities.ErrorModel
	}
}

// No such pod
// swagger:response NoSuchPod
type swagErrNoSuchPod struct {
	// in:body
	Body struct {
		entities.ErrorModel
	}
}

// No such manifest
// swagger:response NoSuchManifest
type swagErrNoSuchManifest struct {
	// in:body
	Body struct {
		entities.ErrorModel
	}
}

// Internal server error
// swagger:response InternalError
type swagInternalError struct {
	// in:body
	Body struct {
		entities.ErrorModel
	}
}

// Conflict error in operation
// swagger:response ConflictError
type swagConflictError struct {
	// in:body
	Body struct {
		entities.ErrorModel
	}
}

// Bad parameter in request
// swagger:response BadParamError
type swagBadParamError struct {
	// in:body
	Body struct {
		entities.ErrorModel
	}
}

// Container already started
// swagger:response ContainerAlreadyStartedError
type swagContainerAlreadyStartedError struct {
	// in:body
	Body struct {
		entities.ErrorModel
	}
}

// Container already stopped
// swagger:response ContainerAlreadyStoppedError
type swagContainerAlreadyStopped struct {
	// in:body
	Body struct {
		entities.ErrorModel
	}
}

// Pod already started
// swagger:response PodAlreadyStartedError
type swagPodAlreadyStartedError struct {
	// in:body
	Body struct {
		entities.ErrorModel
	}
}

// Pod already stopped
// swagger:response PodAlreadyStoppedError
type swagPodAlreadyStopped struct {
	// in:body
	Body struct {
		entities.ErrorModel
	}
}

// Image summary
// swagger:response DockerImageSummary
type swagImageSummary struct {
	// in:body
	Body []entities.ImageSummary
}

// Registries summary
// swagger:response DocsRegistriesList
type swagRegistriesList struct {
	// in:body
	Body entities.ListRegistriesReport
}

// List Containers
// swagger:response DocsListContainer
type swagListContainers struct {
	// in:body
	Body struct {
		// This causes go-swagger to crash
		// handlers.Container
	}
}

// Success
// swagger:response
type ok struct {
	// in:body
	Body struct {
		// example: OK
		ok string
	}
}

// Volume prune response
// swagger:response VolumePruneResponse
type swagVolumePruneResponse struct {
	// in:body
	Body []entities.VolumePruneReport
}

// Volume create response
// swagger:response VolumeCreateResponse
type swagVolumeCreateResponse struct {
	// in:body
	Body struct {
		entities.VolumeConfigResponse
	}
}

// Volume list
// swagger:response VolumeList
type swagVolumeListResponse struct {
	// in:body
	Body []libpod.Volume
}

// Healthcheck
// swagger:response HealthcheckRun
type swagHealthCheckRunResponse struct {
	// in:body
	Body struct {
		define.HealthCheckResults
	}
}

// Version
// swagger:response Version
type swagVersion struct {
	// in:body
	Body struct {
		entities.SystemVersionReport
	}
}

// Disk usage
// swagger:response SystemDiskUse
type swagDiskUseResponse struct {
	// in:body
	Body struct {
		entities.SystemDfReport
	}
}

// Prune report
// swagger:response SystemPruneReport
type swagSystemPruneReport struct {
	// in:body
	Body struct {
		entities.SystemPruneReport
	}
}
