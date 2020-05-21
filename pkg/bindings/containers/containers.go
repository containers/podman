package containers

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/domain/entities"
	sig "github.com/containers/libpod/pkg/signal"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	ErrLostSync = errors.New("lost synchronization with attach multiplexed result")
)

// List obtains a list of containers in local storage.  All parameters to this method are optional.
// The filters are used to determine which containers are listed. The last parameter indicates to only return
// the most recent number of containers.  The pod and size booleans indicate that pod information and rootfs
// size information should also be included.  Finally, the sync bool synchronizes the OCI runtime and
// container state.
func List(ctx context.Context, filters map[string][]string, all *bool, last *int, pod, size, sync *bool) ([]entities.ListContainer, error) { // nolint:typecheck
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
		params.Set("last", strconv.Itoa(*last))
	}
	if pod != nil {
		params.Set("pod", strconv.FormatBool(*pod))
	}
	if size != nil {
		params.Set("size", strconv.FormatBool(*size))
	}
	if sync != nil {
		params.Set("sync", strconv.FormatBool(*sync))
	}
	if filters != nil {
		filterString, err := bindings.FiltersToString(filters)
		if err != nil {
			return nil, err
		}
		params.Set("filters", filterString)
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/containers/json", params)
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
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/prune", params)
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
		params.Set("vols", strconv.FormatBool(*volumes))
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
func Inspect(ctx context.Context, nameOrID string, size *bool) (*define.InspectContainerData, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	if size != nil {
		params.Set("size", strconv.FormatBool(*size))
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/containers/%s/json", params, nameOrID)
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
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/kill", params, nameOrID)
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
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	params := url.Values{}
	if timeout != nil {
		params.Set("t", strconv.Itoa(*timeout))
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
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	params := url.Values{}
	if detachKeys != nil {
		params.Set("detachKeys", *detachKeys)
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/start", params, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

func Stats() {}

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
	response, err := conn.DoRequest(nil, http.MethodGet, "/containers/%s/top", params, nameOrID)
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
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/unpause", nil, nameOrID)
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
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/wait", params, nameOrID)
	if err != nil {
		return exitCode, err
	}
	return exitCode, response.Process(&exitCode)
}

// Exists is a quick, light-weight way to determine if a given container
// exists in local storage.  The nameOrID can be a container name
// or a partial/full ID.
func Exists(ctx context.Context, nameOrID string) (bool, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return false, err
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/containers/%s/exists", nil, nameOrID)
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
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/stop", params, nameOrID)
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
	response, err := conn.DoRequest(nil, http.MethodGet, "/containers/%s/export", params, nameOrID)
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
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/init", nil, nameOrID)
	if err != nil {
		return err
	}
	if response.StatusCode == http.StatusNotModified {
		return errors.Wrapf(define.ErrCtrStateInvalid, "container %s has already been created in runtime", nameOrID)
	}
	return response.Process(nil)
}

// Attach attaches to a running container
func Attach(ctx context.Context, nameOrId string, detachKeys *string, logs, stream *bool, stdin io.Reader, stdout io.Writer, stderr io.Writer, attachReady chan bool) error {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}

	// Do we need to wire in stdin?
	ctnr, err := Inspect(ctx, nameOrId, bindings.PFalse)
	if err != nil {
		return err
	}

	params := url.Values{}
	if detachKeys != nil {
		params.Add("detachKeys", *detachKeys)
	}
	if logs != nil {
		params.Add("logs", fmt.Sprintf("%t", *logs))
	}
	if stream != nil {
		params.Add("stream", fmt.Sprintf("%t", *stream))
	}
	if stdin != nil {
		params.Add("stdin", "true")
	}
	if stdout != nil {
		params.Add("stdout", "true")
	}
	if stderr != nil {
		params.Add("stderr", "true")
	}

	// Unless all requirements are met, don't use "stdin" is a terminal
	file, ok := stdin.(*os.File)
	needTTY := ok && terminal.IsTerminal(int(file.Fd())) && ctnr.Config.Tty
	if needTTY {
		state, err := terminal.MakeRaw(int(file.Fd()))
		if err != nil {
			return err
		}

		logrus.SetFormatter(&rawFormatter{})

		defer func() {
			if err := terminal.Restore(int(file.Fd()), state); err != nil {
				logrus.Errorf("unable to restore terminal: %q", err)
			}
			logrus.SetFormatter(&logrus.TextFormatter{})
		}()

		winChange := make(chan os.Signal, 1)
		signal.Notify(winChange, sig.SIGWINCH)
		winCtx, winCancel := context.WithCancel(ctx)
		defer winCancel()

		go func() {
			// Prime the pump, we need one reset to ensure everything is ready
			winChange <- sig.SIGWINCH
			for {
				select {
				case <-winCtx.Done():
					return
				case <-winChange:
					h, w, err := terminal.GetSize(int(file.Fd()))
					if err != nil {
						logrus.Warnf("failed to obtain TTY size: " + err.Error())
					}

					if err := ResizeContainerTTY(ctx, nameOrId, &h, &w); err != nil {
						logrus.Warnf("failed to resize TTY: " + err.Error())
					}
				}
			}
		}()
	}

	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/attach", params, nameOrId)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	// If we are attaching around a start, we need to "signal"
	// back that we are in fact attached so that started does
	// not execute before we can attach.
	if attachReady != nil {
		attachReady <- true
	}
	if !(response.IsSuccess() || response.IsInformational()) {
		return response.Process(nil)
	}

	if stdin != nil {
		go func() {
			_, err := io.Copy(conn, stdin)
			if err != nil {
				logrus.Error("failed to write input to service: " + err.Error())
			}
		}()
	}

	buffer := make([]byte, 1024)
	if ctnr.Config.Tty {
		// If not multiplex'ed, read from server and write to stdout
		_, err := io.Copy(stdout, response.Body)
		if err != nil {
			return err
		}
	} else {
		for {
			// Read multiplexed channels and write to appropriate stream
			fd, l, err := DemuxHeader(response.Body, buffer)
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return err
			}
			frame, err := DemuxFrame(response.Body, buffer, l)
			if err != nil {
				return err
			}

			switch {
			case fd == 0 && stdin != nil:
				_, err := stdout.Write(frame[0:l])
				if err != nil {
					return err
				}
			case fd == 1 && stdout != nil:
				_, err := stdout.Write(frame[0:l])
				if err != nil {
					return err
				}
			case fd == 2 && stderr != nil:
				_, err := stderr.Write(frame[0:l])
				if err != nil {
					return err
				}
			case fd == 3:
				return fmt.Errorf("error from daemon in stream: %s", frame)
			default:
				return fmt.Errorf("unrecognized input header: %d", fd)
			}
		}
	}
	return nil
}

// DemuxHeader reads header for stream from server multiplexed stdin/stdout/stderr/2nd error channel
func DemuxHeader(r io.Reader, buffer []byte) (fd, sz int, err error) {
	n, err := io.ReadFull(r, buffer[0:8])
	if err != nil {
		return
	}
	if n < 8 {
		err = io.ErrUnexpectedEOF
		return
	}

	fd = int(buffer[0])
	if fd < 0 || fd > 3 {
		err = ErrLostSync
		return
	}

	sz = int(binary.BigEndian.Uint32(buffer[4:8]))
	return
}

// DemuxFrame reads contents for frame from server multiplexed stdin/stdout/stderr/2nd error channel
func DemuxFrame(r io.Reader, buffer []byte, length int) (frame []byte, err error) {
	if len(buffer) < length {
		buffer = append(buffer, make([]byte, length-len(buffer)+1)...)
	}
	n, err := io.ReadFull(r, buffer[0:length])
	if err != nil {
		return nil, nil
	}
	if n < length {
		err = io.ErrUnexpectedEOF
		return
	}

	return buffer[0:length], nil
}

// ResizeContainerTTY sets container's TTY height and width in characters
func ResizeContainerTTY(ctx context.Context, nameOrId string, height *int, width *int) error {
	return resizeTTY(ctx, bindings.JoinURL("containers", nameOrId, "resize"), height, width)
}

// ResizeExecTTY sets session's TTY height and width in characters
func ResizeExecTTY(ctx context.Context, nameOrId string, height *int, width *int) error {
	return resizeTTY(ctx, bindings.JoinURL("exec", nameOrId, "resize"), height, width)
}

// resizeTTY set size of TTY of container
func resizeTTY(ctx context.Context, endpoint string, height *int, width *int) error {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}

	params := url.Values{}
	if height != nil {
		params.Set("h", strconv.Itoa(*height))
	}
	if width != nil {
		params.Set("w", strconv.Itoa(*width))
	}
	rsp, err := conn.DoRequest(nil, http.MethodPost, endpoint, params)
	if err != nil {
		return err
	}
	return rsp.Process(nil)
}

type rawFormatter struct {
	logrus.TextFormatter
}

func (f *rawFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	buffer, err := f.TextFormatter.Format(entry)
	if err != nil {
		return buffer, err
	}
	return append(buffer, '\r'), nil
}
