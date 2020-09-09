package docker

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/docker/distribution/registry/client"
	perrors "github.com/pkg/errors"
)

var (
	// ErrV1NotSupported is returned when we're trying to talk to a
	// docker V1 registry.
	ErrV1NotSupported = errors.New("can't talk to a V1 docker registry")
	// ErrTooManyRequests is returned when the status code returned is 429
	ErrTooManyRequests = errors.New("too many requests to registry")
)

// ErrUnauthorizedForCredentials is returned when the status code returned is 401
type ErrUnauthorizedForCredentials struct { // We only use a struct to allow a type assertion, without limiting the contents of the error otherwise.
	Err error
}

func (e ErrUnauthorizedForCredentials) Error() string {
	return fmt.Sprintf("unable to retrieve auth token: invalid username/password: %s", e.Err.Error())
}

// httpResponseToError translates the https.Response into an error, possibly prefixing it with the supplied context. It returns
// nil if the response is not considered an error.
func httpResponseToError(res *http.Response, context string) error {
	switch res.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusTooManyRequests:
		return ErrTooManyRequests
	case http.StatusUnauthorized:
		err := client.HandleErrorResponse(res)
		return ErrUnauthorizedForCredentials{Err: err}
	default:
		if context != "" {
			context = context + ": "
		}
		return perrors.Errorf("%sinvalid status code from registry %d (%s)", context, res.StatusCode, http.StatusText(res.StatusCode))
	}
}

// registryHTTPResponseToError creates a Go error from an HTTP error response of a docker/distribution
// registry
func registryHTTPResponseToError(res *http.Response) error {
	errResponse := client.HandleErrorResponse(res)
	if e, ok := perrors.Cause(errResponse).(*client.UnexpectedHTTPResponseError); ok {
		response := string(e.Response)
		if len(response) > 50 {
			response = response[:50] + "..."
		}
		errResponse = fmt.Errorf("StatusCode: %d, %s", e.StatusCode, response)
	}
	return errResponse
}
