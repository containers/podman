package bindings

import (
	"encoding/json"
	"io/ioutil"

	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/pkg/errors"
)

var (
	ErrNotImplemented = errors.New("function not implemented")
)

func handleError(data []byte) error {
	e := utils.ErrorModel{}
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	return e
}

func (a APIResponse) Process(unmarshalInto interface{}) error {
	data, err := ioutil.ReadAll(a.Response.Body)
	if err != nil {
		return errors.Wrap(err, "unable to process API response")
	}
	if a.IsSuccess() {
		if unmarshalInto != nil {
			return json.Unmarshal(data, unmarshalInto)
		}
		return nil
	}
	// TODO should we add a debug here with the response code?
	return handleError(data)
}

func CheckResponseCode(inError error) (int, error) {
	e, ok := inError.(utils.ErrorModel)
	if !ok {
		return -1, errors.New("error is not type ErrorModel")
	}
	return e.Code(), nil
}
