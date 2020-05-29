package containers

import (
	"context"
	"net/http"
	"strings"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/bindings"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// ExecCreate creates a new exec session in an existing container.
// The exec session will not be started; that is done with ExecStart.
// Returns ID of new exec session, or an error if one occurred.
func ExecCreate(ctx context.Context, nameOrID string, config *handlers.ExecCreateConfig) (string, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return "", err
	}

	if config == nil {
		return "", errors.Errorf("must provide a configuration for exec session")
	}

	requestJSON, err := json.Marshal(config)
	if err != nil {
		return "", errors.Wrapf(err, "error marshalling exec config to JSON")
	}
	jsonReader := strings.NewReader(string(requestJSON))

	resp, err := conn.DoRequest(jsonReader, http.MethodPost, "/containers/%s/exec", nil, nil, nameOrID)
	if err != nil {
		return "", err
	}

	respStruct := new(handlers.ExecCreateResponse)
	if err := resp.Process(respStruct); err != nil {
		return "", err
	}

	return respStruct.ID, nil
}

// ExecInspect inspects an existing exec session, returning detailed information
// about it.
func ExecInspect(ctx context.Context, sessionID string) (*define.InspectExecSession, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("Inspecting session ID %q", sessionID)

	resp, err := conn.DoRequest(nil, http.MethodGet, "/exec/%s/json", nil, nil, sessionID)
	if err != nil {
		return nil, err
	}

	respStruct := new(define.InspectExecSession)
	if err := resp.Process(respStruct); err != nil {
		return nil, err
	}

	return respStruct, nil
}
