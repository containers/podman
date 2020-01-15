package bindings

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
	if a.Response.StatusCode == http.StatusOK {
		if unmarshalInto != nil {
			return json.Unmarshal(data, unmarshalInto)
		}
		return nil
	}
	// TODO should we add a debug here with the response code?
	return handleError(data)
}

func closeResponseBody(r *http.Response) {
	if r != nil {
		if err := r.Body.Close(); err != nil {
			logrus.Error(errors.Wrap(err, "unable to close response body"))
		}
	}
}
