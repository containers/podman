package utils

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/containers/podman/v5/libpod/events"
	api "github.com/containers/podman/v5/pkg/api/types"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/domain/infra/abi"

	"github.com/containers/podman/v5/pkg/api/handlers"
	"github.com/sirupsen/logrus"

	"github.com/containers/podman/v5/libpod/define"

	"github.com/containers/podman/v5/libpod"
	"github.com/gorilla/schema"
)

type waitQueryDocker struct {
	Condition string `schema:"condition"`
}

type waitQueryLibpod struct {
	Interval   string   `schema:"interval"`
	Conditions []string `schema:"condition"`
}

func WaitContainerDocker(w http.ResponseWriter, r *http.Request) {
	var err error
	ctx := r.Context()

	query := waitQueryDocker{}

	decoder := ctx.Value(api.DecoderKey).(*schema.Decoder)
	if err = decoder.Decode(&query, r.URL.Query()); err != nil {
		Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	interval := time.Millisecond * 250

	condition := "not-running"
	if _, found := r.URL.Query()["condition"]; found {
		condition = query.Condition
		if !isValidDockerCondition(query.Condition) {
			BadRequest(w, "condition", condition, errors.New("not a valid docker condition"))
			return
		}
	}

	name := GetName(r)

	exists, err := containerExists(ctx, name)
	if err != nil {
		InternalServerError(w, err)
		return
	}
	if !exists {
		ContainerNotFound(w, name, define.ErrNoSuchCtr)
		return
	}

	// In docker compatibility mode we have to send headers in advance,
	// otherwise docker client would freeze.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	exitCode, err := waitDockerCondition(ctx, name, interval, condition)
	var errStruct *struct{ Message string }
	if err != nil {
		logrus.Errorf("While waiting on condition: %q", err)
		errStruct = &struct {
			Message string
		}{
			Message: err.Error(),
		}
	}

	responseData := handlers.ContainerWaitOKBody{
		StatusCode: int(exitCode),
		Error:      errStruct,
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	err = enc.Encode(&responseData)
	if err != nil {
		logrus.Errorf("Unable to write json: %q", err)
	}
}

func WaitContainerLibpod(w http.ResponseWriter, r *http.Request) {
	var (
		err      error
		interval = time.Millisecond * 250
	)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := waitQueryLibpod{}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	if _, found := r.URL.Query()["interval"]; found {
		interval, err = time.ParseDuration(query.Interval)
		if err != nil {
			InternalServerError(w, err)
			return
		}
	}

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	containerEngine := &abi.ContainerEngine{Libpod: runtime}
	opts := entities.WaitOptions{
		Conditions: query.Conditions,
		Interval:   interval,
	}
	name := GetName(r)
	reports, err := containerEngine.ContainerWait(r.Context(), []string{name}, opts)
	if err != nil {
		if errors.Is(err, define.ErrNoSuchCtr) {
			ContainerNotFound(w, name, err)
			return
		}
		InternalServerError(w, err)
	}
	if len(reports) != 1 {
		Error(w, http.StatusInternalServerError, fmt.Errorf("the ContainerWait() function returned unexpected count of reports: %d", len(reports)))
		return
	}

	WriteResponse(w, http.StatusOK, strconv.Itoa(int(reports[0].ExitCode)))
}

type containerWaitFn func(conditions ...define.ContainerStatus) (int32, error)

func createContainerWaitFn(ctx context.Context, containerName string, interval time.Duration) containerWaitFn {
	runtime := ctx.Value(api.RuntimeKey).(*libpod.Runtime)
	var containerEngine entities.ContainerEngine = &abi.ContainerEngine{Libpod: runtime}

	return func(conditions ...define.ContainerStatus) (int32, error) {
		var rawConditions []string
		for _, con := range conditions {
			rawConditions = append(rawConditions, con.String())
		}
		opts := entities.WaitOptions{
			Conditions: rawConditions,
			Interval:   interval,
		}
		ctrWaitReport, err := containerEngine.ContainerWait(ctx, []string{containerName}, opts)
		if err != nil {
			return -1, err
		}
		if len(ctrWaitReport) != 1 {
			return -1, fmt.Errorf("the ContainerWait() function returned unexpected count of reports: %d", len(ctrWaitReport))
		}
		return ctrWaitReport[0].ExitCode, ctrWaitReport[0].Error
	}
}

func isValidDockerCondition(cond string) bool {
	switch cond {
	case "next-exit", "removed", "not-running", "":
		return true
	}
	return false
}

func waitDockerCondition(ctx context.Context, containerName string, interval time.Duration, dockerCondition string) (int32, error) {
	containerWait := createContainerWaitFn(ctx, containerName, interval)

	var err error
	var code int32
	switch dockerCondition {
	case "next-exit":
		code, err = waitNextExit(ctx, containerName)
	case "removed":
		code, err = waitRemoved(containerWait)
	case "not-running", "":
		code, err = waitNotRunning(containerWait)
	default:
		panic("not a valid docker condition")
	}
	return code, err
}

var notRunningStates = []define.ContainerStatus{
	define.ContainerStateCreated,
	define.ContainerStateRemoving,
	define.ContainerStateExited,
	define.ContainerStateConfigured,
}

func waitRemoved(ctrWait containerWaitFn) (int32, error) {
	var code int32
	for {
		c, err := ctrWait(define.ContainerStateExited)
		if errors.Is(err, define.ErrNoSuchCtr) {
			// Make sure to wait until the container has been removed.
			break
		}
		if err != nil {
			return code, err
		}
		// If the container doesn't exist, the return code is -1, so
		// only set it in case of success.
		code = c
	}
	return code, nil
}

func waitNextExit(ctx context.Context, containerName string) (int32, error) {
	runtime := ctx.Value(api.RuntimeKey).(*libpod.Runtime)
	containerEngine := &abi.ContainerEngine{Libpod: runtime}
	eventChannel := make(chan *events.Event)
	errChannel := make(chan error)
	opts := entities.EventsOptions{
		EventChan: eventChannel,
		Filter:    []string{"event=died", fmt.Sprintf("container=%s", containerName)},
		Stream:    true,
	}

	// ctx is used to cancel event watching goroutine
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		errChannel <- containerEngine.Events(ctx, opts)
	}()

	evt, ok := <-eventChannel
	if ok {
		if evt.ContainerExitCode != nil {
			return int32(*evt.ContainerExitCode), nil
		}
		return -1, nil
	}
	// if ok == false then containerEngine.Events() has exited
	// it may happen if request was canceled (e.g. client closed connection prematurely) or
	// the server is in process of shutting down
	return -1, <-errChannel
}

func waitNotRunning(ctrWait containerWaitFn) (int32, error) {
	return ctrWait(notRunningStates...)
}

func containerExists(ctx context.Context, name string) (bool, error) {
	runtime := ctx.Value(api.RuntimeKey).(*libpod.Runtime)
	var containerEngine entities.ContainerEngine = &abi.ContainerEngine{Libpod: runtime}

	var ctrExistsOpts entities.ContainerExistsOptions
	ctrExistRep, err := containerEngine.ContainerExists(ctx, name, ctrExistsOpts)
	if err != nil {
		return false, err
	}
	return ctrExistRep.Value, nil
}

// PSTitles merges CAPS headers from ps output. All PS headers are single words, except for
// CAPS. Function compines CAP Headers into single field separated by a space.
func PSTitles(output string) []string {
	var titles []string

	for _, title := range strings.Fields(output) {
		switch title {
		case "AMBIENT", "INHERITED", "PERMITTED", "EFFECTIVE", "BOUNDING":
			{
				titles = append(titles, title+" CAPS")
			}
		case "CAPS":
			continue
		default:
			titles = append(titles, title)
		}
	}
	return titles
}
