package bindings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/containers/podman/v4/pkg/errorhandling"
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
	data, err := io.ReadAll(h.Response.Body)
	if err != nil {
		return fmt.Errorf("unable to process API response: %w", err)
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
		return -1, errors.New("is not type ErrorModel")
	}
}
