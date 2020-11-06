package containers

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/api/handlers"
	"github.com/containers/podman/v2/pkg/bindings"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	ErrLostSync = errors.New("lost synchronization with multiplexed stream")
)

// List obtains a list of containers in local storage.  All parameters to this method are optional.
// The filters are used to determine which containers are listed. The last parameter indicates to only return
// the most recent number of containers.  The pod and size booleans indicate that pod information and rootfs
// size information should also be included.  Finally, the sync bool synchronizes the OCI runtime and
// container state.
func List(ctx context.Context, filters map[string][]string, all *bool, last *int, namespace, size, sync *bool) ([]entities.ListContainer, error) { // nolint:typecheck
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	var containers []entities.ListContainer
	params := url.Values{}
	if all != nil {
		params.Set("all", strconv.FormatBool(*all))
	}
	if last != nil {
		params.Set("limit", strconv.Itoa(*last))
	}
	if size != nil {
		params.Set("size", strconv.FormatBool(*size))
	}
	if sync != nil {
		params.Set("sync", strconv.FormatBool(*sync))
	}
	if namespace != nil {
		params.Set("namespace", strconv.FormatBool(*namespace))
	}
	if filters != nil {
		filterString, err := bindings.FiltersToString(filters)
		if err != nil {
			return nil, err
		}
		params.Set("filters", filterString)
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/containers/json", params, nil)
	if err != nil {
		return containers, err
	}
	return containers, response.Process(&containers)
}

// Prune removes stopped and exited containers from local storage.  The optional filters can be
// used for more granular selection of containers.  The main error returned indicates if there were runtime
// errors like finding containers.  Errors specific to the removal of a container are in the PruneContainerResponse
// structure.
func Prune(ctx context.Context, filters map[string][]string) (*entities.ContainerPruneReport, error) {
	var reports *entities.ContainerPruneReport
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	if filters != nil {
		filterString, err := bindings.FiltersToString(filters)
		if err != nil {
			return nil, err
		}
		params.Set("filters", filterString)
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/prune", params, nil)
	if err != nil {
		return nil, err
	}
	return reports, response.Process(&reports)
}

// Remove removes a container from local storage.  The force bool designates
// that the container should be removed forcibly (example, even it is running).  The volumes
// bool dictates that a container's volumes should also be removed.
func Remove(ctx context.Context, nameOrID string, force, volumes *bool) error {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	params := url.Values{}
	if force != nil {
		params.Set("force", strconv.FormatBool(*force))
	}
	if volumes != nil {
		params.Set("v", strconv.FormatBool(*volumes))
	}
	response, err := conn.DoRequest(nil, http.MethodDelete, "/containers/%s", params, nil, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

// Inspect returns low level information about a Container.  The nameOrID can be a container name
// or a partial/full ID.  The size bool determines whether the size of the container's root filesystem
// should be calculated.  Calculating the size of a container requires extra work from the filesystem and
// is therefore slower.
func Inspect(ctx context.Context, nameOrID string, size *bool) (*define.InspectContainerData, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	if size != nil {
		params.Set("size", strconv.FormatBool(*size))
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/containers/%s/json", params, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	inspect := define.InspectContainerData{}
	return &inspect, response.Process(&inspect)
}

// Kill sends a given signal to a given container.  The signal should be the string
// representation of a signal like 'SIGKILL'. The nameOrID can be a container name
// or a partial/full ID
func Kill(ctx context.Context, nameOrID string, sig string) error {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	params := url.Values{}
	params.Set("signal", sig)
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/kill", params, nil, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)

}

// Pause pauses a given container.  The nameOrID can be a container name
// or a partial/full ID.
func Pause(ctx context.Context, nameOrID string) error {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/pause", nil, nil, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

// Restart restarts a running container. The nameOrID can be a container name
// or a partial/full ID.  The optional timeout specifies the number of seconds to wait
// for the running container to stop before killing it.
func Restart(ctx context.Context, nameOrID string, timeout *int) error {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	params := url.Values{}
	if timeout != nil {
		params.Set("t", strconv.Itoa(*timeout))
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/restart", params, nil, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

// Start starts a non-running container.The nameOrID can be a container name
// or a partial/full ID. The optional parameter for detach keys are to override the default
// detach key sequence.
func Start(ctx context.Context, nameOrID string, detachKeys *string) error {
	logrus.Infof("Going to start container %q", nameOrID)
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	params := url.Values{}
	if detachKeys != nil {
		params.Set("detachKeys", *detachKeys)
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/start", params, nil, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

func Stats(ctx context.Context, containers []string, stream *bool) (chan entities.ContainerStatsReport, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	if stream != nil {
		params.Set("stream", strconv.FormatBool(*stream))
	}
	for _, c := range containers {
		params.Add("containers", c)
	}

	response, err := conn.DoRequest(nil, http.MethodGet, "/containers/stats", params, nil)
	if err != nil {
		return nil, err
	}

	statsChan := make(chan entities.ContainerStatsReport)

	go func() {
		defer close(statsChan)

		dec := json.NewDecoder(response.Body)
		doStream := true
		if stream != nil {
			doStream = *stream
		}

	streamLabel: // label to flatten the scope
		select {
		case <-response.Request.Context().Done():
			return // lost connection - maybe the server quit
		default:
			// fall through and do some work
		}
		var report entities.ContainerStatsReport
		if err := dec.Decode(&report); err != nil {
			report = entities.ContainerStatsReport{Error: err}
		}
		statsChan <- report

		if report.Error != nil || !doStream {
			return
		}
		goto streamLabel
	}()

	return statsChan, nil
}

// Top gathers statistics about the running processes in a container. The nameOrID can be a container name
// or a partial/full ID.  The descriptors allow for specifying which data to collect from the process.
func Top(ctx context.Context, nameOrID string, descriptors []string) ([]string, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params := url.Values{}

	if len(descriptors) > 0 {
		// flatten the slice into one string
		params.Set("ps_args", strings.Join(descriptors, ","))
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/containers/%s/top", params, nil, nameOrID)
	if err != nil {
		return nil, err
	}

	body := handlers.ContainerTopOKBody{}
	if err = response.Process(&body); err != nil {
		return nil, err
	}

	// handlers.ContainerTopOKBody{} returns a slice of slices where each cell in the top table is an item.
	// In libpod land, we're just using a slice with cells being split by tabs, which allows for an idiomatic
	// usage of the tabwriter.
	topOutput := []string{strings.Join(body.Titles, "\t")}
	for _, out := range body.Processes {
		topOutput = append(topOutput, strings.Join(out, "\t"))
	}

	return topOutput, err
}

// Unpause resumes the given paused container.  The nameOrID can be a container name
// or a partial/full ID.
func Unpause(ctx context.Context, nameOrID string) error {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/unpause", nil, nil, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

// Wait blocks until the given container reaches a condition. If not provided, the condition will
// default to stopped.  If the condition is stopped, an exit code for the container will be provided. The
// nameOrID can be a container name or a partial/full ID.
func Wait(ctx context.Context, nameOrID string, condition *define.ContainerStatus) (int32, error) { // nolint
	var exitCode int32
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return exitCode, err
	}
	params := url.Values{}
	if condition != nil {
		params.Set("condition", condition.String())
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/wait", params, nil, nameOrID)
	if err != nil {
		return exitCode, err
	}
	return exitCode, response.Process(&exitCode)
}

// Exists is a quick, light-weight way to determine if a given container
// exists in local storage.  The nameOrID can be a container name
// or a partial/full ID.
func Exists(ctx context.Context, nameOrID string, external bool) (bool, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return false, err
	}
	params := url.Values{}
	params.Set("external", strconv.FormatBool(external))
	response, err := conn.DoRequest(nil, http.MethodGet, "/containers/%s/exists", params, nil, nameOrID)
	if err != nil {
		return false, err
	}
	return response.IsSuccess(), nil
}

// Stop stops a running container.  The timeout is optional. The nameOrID can be a container name
// or a partial/full ID
func Stop(ctx context.Context, nameOrID string, timeout *uint) error {
	params := url.Values{}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	if timeout != nil {
		params.Set("t", strconv.Itoa(int(*timeout)))
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/stop", params, nil, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

// Export creates a tarball of the given name or ID of a container.  It
// requires an io.Writer be provided to write the tarball.
func Export(ctx context.Context, nameOrID string, w io.Writer) error {
	params := url.Values{}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/containers/%s/export", params, nil, nameOrID)
	if err != nil {
		return err
	}
	if response.StatusCode/100 == 2 {
		_, err = io.Copy(w, response.Body)
		return err
	}
	return response.Process(nil)
}

// ContainerInit takes a created container and executes all of the
// preparations to run the container except it will not start
// or attach to the container
func ContainerInit(ctx context.Context, nameOrID string) error {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/init", nil, nil, nameOrID)
	if err != nil {
		return err
	}
	if response.StatusCode == http.StatusNotModified {
		return errors.Wrapf(define.ErrCtrStateInvalid, "container %s has already been created in runtime", nameOrID)
	}
	return response.Process(nil)
}

func ShouldRestart(ctx context.Context, nameOrID string) (bool, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return false, err
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/shouldrestart", nil, nil, nameOrID)
	if err != nil {
		return false, err
	}
	return response.IsSuccess(), nil
}
