package server

import (
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/api/handlers/utils"
)

// No such image
// swagger:response NoSuchImage
type swagErrNoSuchImage struct {
	// in:body
	Body struct {
		utils.ErrorModel
	}
}

// No such container
// swagger:response NoSuchContainer
type swagErrNoSuchContainer struct {
	// in:body
	Body struct {
		utils.ErrorModel
	}
}

// No such exec instance
// swagger:response NoSuchExecInstance
type swagErrNoSuchExecInstance struct {
	// in:body
	Body struct {
		utils.ErrorModel
	}
}

// No such volume
// swagger:response NoSuchVolume
type swagErrNoSuchVolume struct {
	// in:body
	Body struct {
		utils.ErrorModel
	}
}

// No such pod
// swagger:response NoSuchPod
type swagErrNoSuchPod struct {
	// in:body
	Body struct {
		utils.ErrorModel
	}
}

// Internal server error
// swagger:response InternalError
type swagInternalError struct {
	// in:body
	Body struct {
		utils.ErrorModel
	}
}

// Conflict error in operation
// swagger:response ConflictError
type swagConflictError struct {
	// in:body
	Body struct {
		utils.ErrorModel
	}
}

// Bad parameter in request
// swagger:response BadParamError
type swagBadParamError struct {
	// in:body
	Body struct {
		utils.ErrorModel
	}
}

// Container already started
// swagger:response ContainerAlreadyStartedError
type swagContainerAlreadyStartedError struct {
	// in:body
	Body struct {
		utils.ErrorModel
	}
}

// Container already stopped
// swagger:response ContainerAlreadyStoppedError
type swagContainerAlreadyStopped struct {
	// in:body
	Body struct {
		utils.ErrorModel
	}
}

// Pod already started
// swagger:response PodAlreadyStartedError
type swagPodAlreadyStartedError struct {
	// in:body
	Body struct {
		utils.ErrorModel
	}
}

// Pod already stopped
// swagger:response PodAlreadyStoppedError
type swagPodAlreadyStopped struct {
	// in:body
	Body struct {
		utils.ErrorModel
	}
}

// Image summary
// swagger:response DockerImageSummary
type swagImageSummary struct {
	// in:body
	Body []handlers.ImageSummary
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
