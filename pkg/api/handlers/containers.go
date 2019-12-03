package handlers

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/libpod/logs"
	"github.com/containers/libpod/pkg/util"
	"github.com/docker/docker/api/types"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func RemoveContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)

	query := struct {
		Force bool `schema:"force"`
		Vols  bool `schema:"v"`
		Link  bool `schema:"link"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
	if err != nil {
		ContainerNotFound(w, name, err)
		return
	}

	if err := runtime.RemoveContainer(r.Context(), con, query.Force, query.Vols); err != nil {
		InternalServerError(w, err)
		return
	}
	WriteResponse(w, http.StatusNoContent, "")
}

func ListContainers(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	containers, err := runtime.GetAllContainers()
	if err != nil {
		InternalServerError(w, err)
		return
	}

	infoData, err := runtime.Info()
	if err != nil {
		InternalServerError(w, errors.Wrapf(err, "Failed to obtain system info"))
		return
	}

	var list = make([]*Container, len(containers))
	for i, ctnr := range containers {
		api, err := LibpodToContainer(ctnr, infoData)
		if err != nil {
			InternalServerError(w, err)
			return
		}
		list[i] = api
	}
	WriteResponse(w, http.StatusOK, list)
}

func GetContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	name := mux.Vars(r)["name"]
	ctnr, err := runtime.LookupContainer(name)
	if err != nil {
		ContainerNotFound(w, name, err)
		return
	}
	api, err := LibpodToContainerJSON(ctnr)
	if err != nil {
		InternalServerError(w, err)
		return
	}
	WriteResponse(w, http.StatusOK, api)
}

func KillContainer(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value("decorder").(*schema.Decoder)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	// /{version}/containers/(name)/kill
	query := struct {
		Signal syscall.Signal `schema:"signal"`
	}{
		Signal: syscall.SIGKILL,
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
	if err != nil {
		ContainerNotFound(w, name, err)
		return
	}

	state, err := con.State()
	if err != nil {
		InternalServerError(w, err)
		return
	}

	// If the Container is stopped already, send a 409
	if state == define.ContainerStateStopped || state == define.ContainerStateExited {
		Error(w, fmt.Sprintf("Container %s is not running", name), http.StatusConflict, errors.New(fmt.Sprintf("Cannot kill Container %s, it is not running", name)))
		return
	}

	if err := con.Kill(uint(query.Signal)); err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "unable to kill Container %s", name))
		return
	}

	// the kill behavior for docker differs from podman in that they appear to wait
	// for the Container to croak so the exit code is accurate immediately after the
	// kill is sent.  libpod does not.  but we can add a wait here only for the docker
	// side of things and mimic that behavior
	if _, err = con.Wait(); err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "failed to wait for Container %s", name))
		return
	}

	// Success
	WriteResponse(w, http.StatusNoContent, "")
}

func WaitContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	// /{version}/containers/(name)/wait
	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
	if err != nil {
		ContainerNotFound(w, name, err)
		return
	}

	exitCode, err := con.Wait()

	var msg string
	if err != nil {
		msg = err.Error()
	}
	WriteResponse(w, http.StatusOK, ContainerWaitOKBody{
		StatusCode: int(exitCode),
		Error: struct {
			Message string
		}{
			Message: msg,
		},
	})
}

func StopContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)

	// /{version}/containers/(name)/stop
	query := struct {
		Timeout int `schema:"t"`
	}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
	if err != nil {
		ContainerNotFound(w, name, err)
		return
	}

	state, err := con.State()
	if err != nil {
		InternalServerError(w, errors.Wrapf(err, "unable to get state for Container %s", name))
		return
	}

	// If the Container is stopped already, send a 302
	if state == define.ContainerStateStopped || state == define.ContainerStateExited {
		Error(w, http.StatusText(http.StatusNotModified), http.StatusNotModified,
			errors.Wrapf(err, fmt.Sprintf("Container %s is already stopped ", name)))
		return
	}

	var stopError error
	if _, found := mux.Vars(r)["t"]; found {
		stopError = con.StopWithTimeout(uint(query.Timeout))
	} else {
		stopError = con.Stop()
	}
	if stopError != nil {
		InternalServerError(w, errors.Wrapf(err, "failed to stop %s", name))
		return
	}

	// Success
	WriteResponse(w, http.StatusNoContent, "")
}

func PauseContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	// /{version}/containers/(name)/pause
	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
	if err != nil {
		ContainerNotFound(w, name, err)
		return
	}

	// the api does not error if the Container is already paused, so just into it
	if err := con.Pause(); err != nil {
		InternalServerError(w, err)
		return
	}
	// Success
	WriteResponse(w, http.StatusNoContent, "")
}

func PruneContainers(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	containers, err := runtime.GetAllContainers()
	if err != nil {
		InternalServerError(w, err)
		return
	}

	deletedContainers := []string{}
	var spaceReclaimed uint64
	for _, ctnr := range containers {
		// Only remove stopped or exit'ed containers.
		state, err := ctnr.State()
		if err != nil {
			InternalServerError(w, err)
			return
		}
		switch state {
		case define.ContainerStateStopped, define.ContainerStateExited:
		default:
			continue
		}

		deletedContainers = append(deletedContainers, ctnr.ID())
		cSize, err := ctnr.RootFsSize()
		if err != nil {
			InternalServerError(w, err)
			return
		}
		spaceReclaimed += uint64(cSize)

		err = runtime.RemoveContainer(context.Background(), ctnr, false, false)
		if err != nil && !(errors.Cause(err) == define.ErrCtrRemoved) {
			InternalServerError(w, err)
			return
		}
	}
	report := types.ContainersPruneReport{
		ContainersDeleted: deletedContainers,
		SpaceReclaimed:    spaceReclaimed,
	}
	WriteResponse(w, http.StatusOK, report)
}

func UnpauseContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	// /{version}/containers/(name)/unpause
	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
	if err != nil {
		ContainerNotFound(w, name, err)
		return
	}

	// the api does not error if the Container is already paused, so just into it
	if err := con.Unpause(); err != nil {
		InternalServerError(w, err)
		return
	}

	// Success
	WriteResponse(w, http.StatusNoContent, "")
}

func StartContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
	if err != nil {
		ContainerNotFound(w, name, err)
		return
	}

	state, err := con.State()
	if err != nil {
		InternalServerError(w, err)
		return
	}
	if state == define.ContainerStateRunning {
		msg := fmt.Sprintf("Container %s is already running", name)
		Error(w, msg, http.StatusNotModified, errors.New(msg))
		return
	}
	if err := con.Start(r.Context(), false); err != nil {
		InternalServerError(w, err)
		return
	}
	WriteResponse(w, http.StatusNoContent, "")
}

func RestartContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	// /{version}/containers/(name)/restart
	query := struct {
		Timeout int `schema:"t"`
	}{
		// Override golang default values for types
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
	if err != nil {
		ContainerNotFound(w, name, err)
		return
	}

	state, err := con.State()
	if err != nil {
		InternalServerError(w, err)
		return
	}

	// FIXME: This is not in the swagger.yml...
	// If the Container is stopped already, send a 409
	if state == define.ContainerStateStopped || state == define.ContainerStateExited {
		msg := fmt.Sprintf("Container %s is not running", name)
		Error(w, msg, http.StatusConflict, errors.New(msg))
		return
	}

	timeout := con.StopTimeout()
	if _, found := mux.Vars(r)["t"]; found {
		timeout = uint(query.Timeout)
	}

	if err := con.RestartWithTimeout(r.Context(), timeout); err != nil {
		InternalServerError(w, err)
		return
	}

	// Success
	WriteResponse(w, http.StatusNoContent, "")
}

func LogsFromContainer(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	query := struct {
		Follow     bool   `schema:"follow"`
		Stdout     bool   `schema:"stdout"`
		Stderr     bool   `schema:"stderr"`
		Since      string `schema:"since"`
		Until      string `schema:"until"`
		Timestamps bool   `schema:"timestamps"`
		Tail       string `schema:"tail"`
	}{
		Tail: "all",
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	if !(query.Stdout || query.Stderr) {
		msg := fmt.Sprintf("%s: you must choose at least one stream", http.StatusText(http.StatusBadRequest))
		Error(w, msg, http.StatusBadRequest, errors.Errorf("%s for %s", msg, r.URL.String()))
		return
	}

	name := mux.Vars(r)["name"]
	ctnr, err := runtime.LookupContainer(name)
	if err != nil {
		ContainerNotFound(w, name, err)
		return
	}

	var tail int64 = -1
	if query.Tail != "all" {
		tail, err = strconv.ParseInt(query.Tail, 0, 64)
		if err != nil {
			BadRequest(w, "tail", query.Tail, err)
			return
		}
	}

	var since time.Time
	if _, found := mux.Vars(r)["since"]; found {
		since, err = util.ParseInputTime(query.Since)
		if err != nil {
			BadRequest(w, "since", query.Since, err)
			return
		}
	}

	var until time.Time
	if _, found := mux.Vars(r)["until"]; found {
		since, err = util.ParseInputTime(query.Until)
		if err != nil {
			BadRequest(w, "until", query.Until, err)
			return
		}
	}

	options := &logs.LogOptions{
		Details:    true,
		Follow:     query.Follow,
		Since:      since,
		Tail:       tail,
		Timestamps: query.Timestamps,
	}

	var wg sync.WaitGroup
	options.WaitGroup = &wg

	logChannel := make(chan *logs.LogLine, tail+1)
	if err := runtime.Log([]*libpod.Container{ctnr}, options, logChannel); err != nil {
		InternalServerError(w, errors.Wrapf(err, "Failed to obtain logs for Container '%s'", name))
		return
	}
	go func() {
		wg.Wait()
		close(logChannel)
	}()

	w.WriteHeader(http.StatusOK)
	var builder strings.Builder
	for ok := true; ok; ok = query.Follow {
		for line := range logChannel {
			if _, found := mux.Vars(r)["until"]; found {
				if line.Time.After(until) {
					break
				}
			}

			// Reset variables we're ready to loop again
			builder.Reset()
			header := [8]byte{}

			switch line.Device {
			case "stdout":
				if !query.Stdout {
					continue
				}
				header[0] = 1
			case "stderr":
				if !query.Stderr {
					continue
				}
				header[0] = 2
			default:
				// Logging and moving on is the best we can do here. We may have already sent
				// a Status and Content-Type to client therefore we can no longer report an error.
				log.Infof("Unknown Device type '%s' in log file from Container %s", line.Device, ctnr.ID())
				continue
			}

			if query.Timestamps {
				builder.WriteString(line.Time.Format(time.RFC3339))
				builder.WriteRune(' ')
			}
			builder.WriteString(line.Msg)

			// Build header and output entry
			binary.BigEndian.PutUint32(header[4:], uint32(len(header)+builder.Len()))
			if _, err := w.Write(header[:]); err != nil {
				log.Errorf("unable to write log output header: %q", err)
			}
			if _, err := fmt.Fprint(w, builder.String()); err != nil {
				log.Errorf("unable to write builder string: %q", err)
			}

			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}

func ContainerExists(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	// /containers/libpod/{name:..*}/exists
	name := mux.Vars(r)["name"]
	_, err := runtime.LookupContainer(name)
	if err != nil {
		ContainerNotFound(w, name, err)
		return
	}
	WriteResponse(w, http.StatusOK, http.StatusText(http.StatusOK))
}
