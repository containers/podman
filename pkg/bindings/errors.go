package bindings

import (
	"encoding/json"
	"io/ioutil"

	"github.com/containers/podman/v3/pkg/errorhandling"
	"github.com/pkg/errors"
)

var (
	ErrNotImplemented = errors.New("function not implemented")
)

func handleError(data []byte) error {
	e := errorhandling.ErrorModel{}
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	return e
}

func (h APIResponse) Process(unmarshalInto interface{}) error {
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
	// TODO should we add a debug here with the response code?
	return handleError(data)
}

func CheckResponseCode(inError error) (int, error) {
	e, ok := inError.(errorhandling.ErrorModel)
	if !ok {
		return -1, errors.New("error is not type ErrorModel")
	}
	return e.Code(), nil
}
