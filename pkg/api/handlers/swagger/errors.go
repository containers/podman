//go:build !remote

//nolint:unused // these types are used to wire generated swagger to API code
package swagger

import (
	"github.com/containers/podman/v5/pkg/errorhandling"
)

// Error model embedded in swagger:response to aid in documentation generation

// No such image
// swagger:response
type imageNotFound struct {
	// in:body
	Body errorhandling.ErrorModel
}

// No such container
// swagger:response
type containerNotFound struct {
	// in:body
	Body errorhandling.ErrorModel
}

// No such network
// swagger:response
type networkNotFound struct {
	// in:body
	Body errorhandling.ErrorModel
}

// Network is already connected and container is running or transitioning to the running state ('initialized')
// swagger:response
type networkConnectedError struct {
	// in:body
	Body errorhandling.ErrorModel
}

// No such exec instance
// swagger:response
type execSessionNotFound struct {
	// in:body
	Body errorhandling.ErrorModel
}

// No such volume
// swagger:response
type volumeNotFound struct {
	// in:body
	Body errorhandling.ErrorModel
}

// No such pod
// swagger:response
type podNotFound struct {
	// in:body
	Body errorhandling.ErrorModel
}

// No such manifest
// swagger:response
type manifestNotFound struct {
	// in:body
	Body errorhandling.ErrorModel
}

// Internal server error
// swagger:response
type internalError struct {
	// in:body
	Body errorhandling.ErrorModel
}

// Conflict error in operation
// swagger:response
type conflictError struct {
	// in:body
	Body errorhandling.ErrorModel
}

// Bad parameter in request
// swagger:response
type badParamError struct {
	// in:body
	Body errorhandling.ErrorModel
}

// Container already started
// swagger:response
type containerAlreadyStartedError struct {
	// in:body
	Body errorhandling.ErrorModel
}

// Container already stopped
// swagger:response
type containerAlreadyStoppedError struct {
	// in:body
	Body errorhandling.ErrorModel
}

// Pod already started
// swagger:response
type podAlreadyStartedError struct {
	// in:body
	Body errorhandling.ErrorModel
}

// Pod already stopped
// swagger:response
type podAlreadyStoppedError struct {
	// in:body
	Body errorhandling.ErrorModel
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
