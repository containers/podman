package pods

import (
	"context"
	"net/http"
	"strconv"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/bindings"
)

func CreatePod() error {
	// TODO
	return bindings.ErrNotImplemented
}

// Exists is a lightweight method to determine if a pod exists in local storage
func Exists(ctx context.Context, nameOrID string) (bool, error) {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return false, err
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/pods/%s/exists", nil, nameOrID)
	if err != nil {
		return false, err
	}
	return response.IsSuccess(), nil
}

// Inspect returns low-level information about the given pod.
func Inspect(ctx context.Context, nameOrID string) (*libpod.PodInspect, error) {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return nil, err
	}
	inspect := libpod.PodInspect{}
	response, err := conn.DoRequest(nil, http.MethodGet, "/pods/%s/json", nil, nameOrID)
	if err != nil {
		return &inspect, err
	}
	return &inspect, response.Process(&inspect)
}

// Kill sends a SIGTERM to all the containers in a pod.  The optional signal parameter
// can be used to override  SIGTERM.
func Kill(ctx context.Context, nameOrID string, signal *string) error {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return err
	}
	params := make(map[string]string)
	if signal != nil {
		params["signal"] = *signal
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/pods/%s/kill", params, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

// Pause pauses all running containers in a given pod.
func Pause(ctx context.Context, nameOrID string) error {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return err
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/pods/%s/pause", nil, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

// Prune removes all non-running pods in local storage.
func Prune(ctx context.Context) error {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return err
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/pods/prune", nil)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

// List returns all pods in local storage.  The optional filters parameter can
// be used to refine which pods should be listed.
func List(ctx context.Context, filters map[string][]string) (*[]libpod.PodInspect, error) {
	var (
		inspect []libpod.PodInspect
	)
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return nil, err
	}
	params := make(map[string]string)
	if filters != nil {
		stringFilter, err := bindings.FiltersToHTML(filters)
		if err != nil {
			return nil, err
		}
		params["filters"] = stringFilter
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/pods/json", params)
	if err != nil {
		return &inspect, err
	}
	return &inspect, response.Process(&inspect)
}

// Restart restarts all containers in a pod.
func Restart(ctx context.Context, nameOrID string) error {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return err
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/pods/%s/restart", nil, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

// Remove deletes a Pod from from local storage. The optional force parameter denotes
// that the Pod can be removed even if in a running state.
func Remove(ctx context.Context, nameOrID string, force *bool) error {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return err
	}
	params := make(map[string]string)
	if force != nil {
		params["force"] = strconv.FormatBool(*force)
	}
	response, err := conn.DoRequest(nil, http.MethodDelete, "/pods/%s", params, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

// Start starts all containers in a pod.
func Start(ctx context.Context, nameOrID string) error {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return err
	}
	response, err := conn.DoRequest(nil, http.MethodDelete, "/pods/%s/start", nil, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

func Stats() error {
	// TODO
	return bindings.ErrNotImplemented
}

// Stop stops all containers in a Pod. The optional timeout parameter can be
// used to override the timeout before the container is killed.
func Stop(ctx context.Context, nameOrID string, timeout *int) error {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return err
	}
	params := make(map[string]string)
	if timeout != nil {
		params["t"] = strconv.Itoa(*timeout)
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/pods/%s/stop", params, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

func Top() error {
	// TODO
	return bindings.ErrNotImplemented // nolint:typecheck
}

// Unpause unpauses all paused containers in a Pod.
func Unpause(ctx context.Context, nameOrID string) error {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return err
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/pods/%s/unpause", nil, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}
