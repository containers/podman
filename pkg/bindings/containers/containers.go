package containers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/bindings"
)

// List obtains a list of containers in local storage.  All parameters to this method are optional.
// The filters are used to determine which containers are listed. The last parameter indicates to only return
// the most recent number of containers.  The pod and size booleans indicate that pod information and rootfs
// size information should also be included.  Finally, the sync bool synchronizes the OCI runtime and
// container state.
func List(ctx context.Context, filters map[string][]string, last *int, pod, size, sync *bool) ([]*shared.PsContainerOutput, error) { // nolint:typecheck
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return nil, err
	}
	var images []*shared.PsContainerOutput
	params := make(map[string]string)
	if last != nil {
		params["last"] = strconv.Itoa(*last)
	}
	if pod != nil {
		params["pod"] = strconv.FormatBool(*pod)
	}
	if size != nil {
		params["size"] = strconv.FormatBool(*size)
	}
	if sync != nil {
		params["sync"] = strconv.FormatBool(*sync)
	}
	if filters != nil {
		filterString, err := bindings.FiltersToHTML(filters)
		if err != nil {
			return nil, err
		}
		params["filters"] = filterString
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/containers/json", params)
	if err != nil {
		return images, err
	}
	return images, response.Process(nil)
}

// Prune removes stopped and exited containers from local storage.  The optional filters can be
// used for more granular selection of containers.  The main error returned indicates if there were runtime
// errors like finding containers.  Errors specific to the removal of a container are in the PruneContainerResponse
// structure.
func Prune(ctx context.Context, filters map[string][]string) ([]string, error) {
	var (
		pruneResponse []string
	)
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return nil, err
	}
	params := make(map[string]string)
	if filters != nil {
		filterString, err := bindings.FiltersToHTML(filters)
		if err != nil {
			return nil, err
		}
		params["filters"] = filterString
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/prune", params)
	if err != nil {
		return pruneResponse, err
	}
	return pruneResponse, response.Process(pruneResponse)
}

// Remove removes a container from local storage.  The force bool designates
// that the container should be removed forcibly (example, even it is running).  The volumes
// bool dictates that a container's volumes should also be removed.
func Remove(ctx context.Context, nameOrID string, force, volumes *bool) error {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return err
	}
	params := make(map[string]string)
	if force != nil {
		params["force"] = strconv.FormatBool(*force)
	}
	if volumes != nil {
		params["vols"] = strconv.FormatBool(*volumes)
	}
	response, err := conn.DoRequest(nil, http.MethodDelete, "/containers/%s", params, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

// Inspect returns low level information about a Container.  The nameOrID can be a container name
// or a partial/full ID.  The size bool determines whether the size of the container's root filesystem
// should be calculated.  Calculating the size of a container requires extra work from the filesystem and
// is therefore slower.
func Inspect(ctx context.Context, nameOrID string, size *bool) (*libpod.InspectContainerData, error) {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return nil, err
	}
	params := make(map[string]string)
	if size != nil {
		params["size"] = strconv.FormatBool(*size)
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/containers/%s/json", params, nameOrID)
	if err != nil {
		return nil, err
	}
	inspect := libpod.InspectContainerData{}
	return &inspect, response.Process(&inspect)
}

// Kill sends a given signal to a given container.  The signal should be the string
// representation of a signal like 'SIGKILL'. The nameOrID can be a container name
// or a partial/full ID
func Kill(ctx context.Context, nameOrID string, signal string) error {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return err
	}
	params := make(map[string]string)
	params["signal"] = signal
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/kill", params, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)

}
func Logs() {}

// Pause pauses a given container.  The nameOrID can be a container name
// or a partial/full ID.
func Pause(ctx context.Context, nameOrID string) error {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return err
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/pause", nil, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

// Restart restarts a running container. The nameOrID can be a container name
// or a partial/full ID.  The optional timeout specifies the number of seconds to wait
// for the running container to stop before killing it.
func Restart(ctx context.Context, nameOrID string, timeout *int) error {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return err
	}
	params := make(map[string]string)
	if timeout != nil {
		params["t"] = strconv.Itoa(*timeout)
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/restart", params, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

// Start starts a non-running container.The nameOrID can be a container name
// or a partial/full ID. The optional parameter for detach keys are to override the default
// detach key sequence.
func Start(ctx context.Context, nameOrID string, detachKeys *string) error {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return err
	}
	params := make(map[string]string)
	if detachKeys != nil {
		params["detachKeys"] = *detachKeys
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/start", params, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

func Stats() {}
func Top()   {}

// Unpause resumes the given paused container.  The nameOrID can be a container name
// or a partial/full ID.
func Unpause(ctx context.Context, nameOrID string) error {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return err
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/unpause", nil, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

// Wait blocks until the given container exits and returns its exit code. The nameOrID can be a container name
// or a partial/full ID.
func Wait(ctx context.Context, nameOrID string) (int32, error) {
	var exitCode int32
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return exitCode, err
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "containers/%s/wait", nil, nameOrID)
	if err != nil {
		return exitCode, err
	}
	return exitCode, response.Process(&exitCode)
}

// Exists is a quick, light-weight way to determine if a given container
// exists in local storage.  The nameOrID can be a container name
// or a partial/full ID.
func Exists(ctx context.Context, nameOrID string) (bool, error) {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return false, err
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "containers/%s/exists", nil, nameOrID)
	if err != nil {
		return false, err
	}
	return response.IsSuccess(), nil
}

// Stop stops a running container.  The timeout is optional. The nameOrID can be a container name
// or a partial/full ID
func Stop(ctx context.Context, nameOrID string, timeout *int) error {
	params := make(map[string]string)
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return err
	}
	if timeout != nil {
		params["t"] = strconv.Itoa(*timeout)
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/stop", params, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}
