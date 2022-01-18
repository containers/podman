package bindings

import (
	"encoding/json"
	"io/ioutil"

	"github.com/containers/podman/v4/pkg/errorhandling"
	"github.com/pkg/errors"
)

var (
	ErrNotImplemented = errors.New("function not implemented")
)

func handleError(data []byte, unmarshalErrorInto interface{}) error {
	if err := json.Unmarshal(data, unmarshalErrorInto); err != nil {
		return err
	}
	return unmarshalErrorInto.(error)
}

// Process drains the response body, and processes the HTTP status code
// Note: Closing the response.Body is left to the caller
func (h APIResponse) Process(unmarshalInto interface{}) error {
	return h.ProcessWithError(unmarshalInto, &errorhandling.ErrorModel{})
}

// ProcessWithError drains the response body, and processes the HTTP status code
// Note: Closing the response.Body is left to the caller
func (h APIResponse) ProcessWithError(unmarshalInto interface{}, unmarshalErrorInto interface{}) error {
	data, err := ioutil.ReadAll(h.Response.Body)
	if err != nil {
		return errors.Wrap(err, "unable to process API response")
	}
	if h.IsSuccess() || h.IsRedirection() {
		if unmarshalInto != nil {
			return json.Unmarshal(data, unmarshalInto)
		}
		return nil
	}

	if h.IsConflictError() {
		return handleError(data, unmarshalErrorInto)
	}

	// TODO should we add a debug here with the response code?
	return handleError(data, &errorhandling.ErrorModel{})
}

func CheckResponseCode(inError error) (int, error) {
	switch e := inError.(type) {
	case *errorhandling.ErrorModel:
		return e.Code(), nil
	case *errorhandling.PodConflictErrorModel:
		return e.Code(), nil
	default:
		return -1, errors.New("error is not type ErrorModel")
	}
}
