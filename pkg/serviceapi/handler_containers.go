package serviceapi

import (
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
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func (s *APIServer) registerContainersHandlers(r *mux.Router) error {
	r.HandleFunc(versionedPath("/containers/create"), s.serviceHandler(s.createContainer)).Methods("POST")
	r.HandleFunc(versionedPath("/containers/json"), s.serviceHandler(s.listContainers)).Methods("GET")
	r.HandleFunc(versionedPath("/containers/{name:..*}"), s.serviceHandler(s.removeContainer)).Methods("DELETE")
	r.HandleFunc(versionedPath("/containers/{name:..*}/json"), s.serviceHandler(s.container)).Methods("GET")
	r.HandleFunc(versionedPath("/containers/{name:..*}/kill"), s.serviceHandler(s.killContainer)).Methods("POST")
	r.HandleFunc(versionedPath("/containers/{name:..*}/logs"), s.serviceHandler(s.logsFromContainer)).Methods("GET")
	r.HandleFunc(versionedPath("/containers/{name:..*}/pause"), s.serviceHandler(s.pauseContainer)).Methods("POST")
	r.HandleFunc(versionedPath("/containers/{name:..*}/rename"), s.serviceHandler(s.unsupportedHandler)).Methods("POST")
	r.HandleFunc(versionedPath("/containers/{name:..*}/restart"), s.serviceHandler(s.restartContainer)).Methods("POST")
	r.HandleFunc(versionedPath("/containers/{name:..*}/start"), s.serviceHandler(s.startContainer)).Methods("POST")
	r.HandleFunc(versionedPath("/containers/{name:..*}/stats"), s.serviceHandler(s.statsContainer)).Methods("GET")
	r.HandleFunc(versionedPath("/containers/{name:..*}/stop"), s.serviceHandler(s.stopContainer)).Methods("POST")
	r.HandleFunc(versionedPath("/containers/{name:..*}/top"), s.serviceHandler(s.topContainer)).Methods("GET")
	r.HandleFunc(versionedPath("/containers/{name:..*}/unpause"), s.serviceHandler(s.unpauseContainer)).Methods("POST")
	r.HandleFunc(versionedPath("/containers/{name:..*}/wait"), s.serviceHandler(s.waitContainer)).Methods("POST")

	// libpod endpoints
	r.HandleFunc(versionedPath("/libpod/containers/{name:..*}/exists"), s.serviceHandler(s.containerExists))
	return nil
}

func (s *APIServer) removeContainer(w http.ResponseWriter, r *http.Request) {
	query := struct {
		Force bool `schema:"force"`
		Vols  bool `schema:"v"`
		Link  bool `schema:"link"`
	}{
		// override any golang type defaults
	}
	if err := s.Decode(&query, r.URL.Query()); err != nil {
		Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	name := mux.Vars(r)["name"]
	con, err := s.Runtime.LookupContainer(name)
	if err != nil {
		containerNotFound(w, name, err)
		return
	}

	if err := s.Runtime.RemoveContainer(s.Context, con, query.Force, query.Vols); err != nil {
		internalServerError(w, err)
		return
	}
	s.WriteResponse(w, http.StatusNoContent, "")
}

func (s *APIServer) listContainers(w http.ResponseWriter, r *http.Request) {
	containers, err := s.Runtime.GetAllContainers()
	if err != nil {
		internalServerError(w, err)
		return
	}

	infoData, err := s.Runtime.Info()
	if err != nil {
		internalServerError(w, errors.Wrapf(err, "Failed to obtain system info"))
		return
	}

	var list = make([]*Container, len(containers))
	for i, ctnr := range containers {
		api, err := LibpodToContainer(ctnr, infoData)
		if err != nil {
			internalServerError(w, err)
			return
		}
		list[i] = api
	}
	s.WriteResponse(w, http.StatusOK, list)
}

func (s *APIServer) container(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	ctnr, err := s.Runtime.LookupContainer(name)
	if err != nil {
		containerNotFound(w, name, err)
		return
	}

	infoData, err := s.Runtime.Info()
	if err != nil {
		internalServerError(w, errors.Wrapf(err, "Failed to obtain system info"))
		return
	}

	api, err := LibpodToContainer(ctnr, infoData)
	if err != nil {
		internalServerError(w, err)
		return
	}
	s.WriteResponse(w, http.StatusOK, api)
}

func (s *APIServer) killContainer(w http.ResponseWriter, r *http.Request) {
	// /{version}/containers/(name)/kill
	query := struct {
		Signal syscall.Signal `schema:"signal"`
	}{
		Signal: syscall.SIGKILL,
	}
	if err := s.Decode(&query, r.URL.Query()); err != nil {
		Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	name := mux.Vars(r)["name"]
	con, err := s.Runtime.LookupContainer(name)
	if err != nil {
		containerNotFound(w, name, err)
		return
	}

	state, err := con.State()
	if err != nil {
		internalServerError(w, err)
		return
	}

	// If the container is stopped already, send a 409
	if state == define.ContainerStateStopped || state == define.ContainerStateExited {
		Error(w, fmt.Sprintf("Container %s is not running", name), http.StatusConflict, errors.New(fmt.Sprintf("Cannot kill container %s, it is not running", name)))
		return
	}

	if err := con.Kill(uint(query.Signal)); err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "unable to kill container %s", name))
		return
	}
	// Success
	s.WriteResponse(w, http.StatusNoContent, "")
}

func (s *APIServer) waitContainer(w http.ResponseWriter, r *http.Request) {
	// /{version}/containers/(name)/wait
	name := mux.Vars(r)["name"]
	con, err := s.Runtime.LookupContainer(name)
	if err != nil {
		containerNotFound(w, name, err)
		return
	}

	exitCode, err := con.Wait()

	var msg string
	if err != nil {
		msg = err.Error()
	}
	s.WriteResponse(w, http.StatusOK, ContainerWaitOKBody{
		StatusCode: int(exitCode),
		Error: struct {
			Message string
		}{
			Message: msg,
		},
	})
}

func (s *APIServer) stopContainer(w http.ResponseWriter, r *http.Request) {
	// /{version}/containers/(name)/stop
	query := struct {
		Timeout int `schema:"t"`
	}{
		// override any golang type defaults
	}
	if err := s.Decode(&query, r.URL.Query()); err != nil {
		Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	name := mux.Vars(r)["name"]
	con, err := s.Runtime.LookupContainer(name)
	if err != nil {
		containerNotFound(w, name, err)
		return
	}

	state, err := con.State()
	if err != nil {
		internalServerError(w, errors.Wrapf(err, "unable to get state for container %s", name))
		return
	}

	// If the container is stopped already, send a 302
	if state == define.ContainerStateStopped || state == define.ContainerStateExited {
		Error(w, http.StatusText(http.StatusNotModified), http.StatusNotModified,
			errors.Wrapf(err, fmt.Sprintf("container %s is already stopped ", name)))
		return
	}

	var stopError error
	if _, found := mux.Vars(r)["t"]; found {
		stopError = con.StopWithTimeout(uint(query.Timeout))
	} else {
		stopError = con.Stop()
	}
	if stopError != nil {
		internalServerError(w, errors.Wrapf(err, "failed to stop %s", name))
		return
	}

	// Success
	s.WriteResponse(w, http.StatusNoContent, "")
}

func (s *APIServer) pauseContainer(w http.ResponseWriter, r *http.Request) {
	// /{version}/containers/(name)/pause
	name := mux.Vars(r)["name"]
	con, err := s.Runtime.LookupContainer(name)
	if err != nil {
		containerNotFound(w, name, err)
		return
	}

	// the api does not error if the container is already paused, so just into it
	if err := con.Pause(); err != nil {
		internalServerError(w, err)
		return
	}
	// Success
	s.WriteResponse(w, http.StatusNoContent, "")
}

func (s *APIServer) unpauseContainer(w http.ResponseWriter, r *http.Request) {
	// /{version}/containers/(name)/unpause
	name := mux.Vars(r)["name"]
	con, err := s.Runtime.LookupContainer(name)
	if err != nil {
		containerNotFound(w, name, err)
		return
	}

	// the api does not error if the container is already paused, so just into it
	if err := con.Unpause(); err != nil {
		internalServerError(w, err)
		return
	}

	// Success
	s.WriteResponse(w, http.StatusNoContent, "")
}

func (s *APIServer) startContainer(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	con, err := s.Runtime.LookupContainer(name)
	if err != nil {
		containerNotFound(w, name, err)
		return
	}

	state, err := con.State()
	if err != nil {
		internalServerError(w, err)
		return
	}
	if state == define.ContainerStateRunning {
		msg := fmt.Sprintf("Container %s is already running", name)
		Error(w, msg, http.StatusNotModified, errors.New(msg))
		return
	}
	if err := con.Start(s.Context, false); err != nil {
		internalServerError(w, err)
		return
	}
	s.WriteResponse(w, http.StatusNoContent, "")
}

func (s *APIServer) restartContainer(w http.ResponseWriter, r *http.Request) {
	// /{version}/containers/(name)/restart
	query := struct {
		Timeout int `schema:"t"`
	}{
		// Override golang default values for types
	}
	if err := s.Decode(&query, r.URL.Query()); err != nil {
		Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	name := mux.Vars(r)["name"]
	con, err := s.Runtime.LookupContainer(name)
	if err != nil {
		containerNotFound(w, name, err)
		return
	}

	state, err := con.State()
	if err != nil {
		internalServerError(w, err)
		return
	}

	// FIXME: This is not in the swagger.yml...
	// If the container is stopped already, send a 409
	if state == define.ContainerStateStopped || state == define.ContainerStateExited {
		msg := fmt.Sprintf("Container %s is not running", name)
		Error(w, msg, http.StatusConflict, errors.New(msg))
		return
	}

	timeout := con.StopTimeout()
	if _, found := mux.Vars(r)["t"]; found {
		timeout = uint(query.Timeout)
	}

	if err := con.RestartWithTimeout(s.Context, timeout); err != nil {
		internalServerError(w, err)
		return
	}

	// Success
	s.WriteResponse(w, http.StatusNoContent, "")
}

func (s *APIServer) logsFromContainer(w http.ResponseWriter, r *http.Request) {
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
	if err := s.Decode(&query, r.URL.Query()); err != nil {
		Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	if !(query.Stdout || query.Stderr) {
		msg := fmt.Sprintf("%s: you must choose at least one stream", http.StatusText(http.StatusBadRequest))
		Error(w, msg, http.StatusBadRequest, errors.Errorf("%s for %s", msg, r.URL.String()))
		return
	}

	name := mux.Vars(r)["name"]
	ctnr, err := s.Runtime.LookupContainer(name)
	if err != nil {
		containerNotFound(w, name, err)
		return
	}

	var tail int64 = -1
	if query.Tail != "all" {
		tail, err = strconv.ParseInt(query.Tail, 0, 64)
		if err != nil {
			badRequest(w, "tail", query.Tail, err)
			return
		}
	}

	var since time.Time
	if _, found := mux.Vars(r)["since"]; found {
		since, err = util.ParseInputTime(query.Since)
		if err != nil {
			badRequest(w, "since", query.Since, err)
			return
		}
	}

	var until time.Time
	if _, found := mux.Vars(r)["until"]; found {
		since, err = util.ParseInputTime(query.Until)
		if err != nil {
			badRequest(w, "until", query.Until, err)
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
	if err := s.Runtime.Log([]*libpod.Container{ctnr}, options, logChannel); err != nil {
		internalServerError(w, errors.Wrapf(err, "Failed to obtain logs for container '%s'", name))
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
				log.Infof("Unknown Device type '%s' in log file from container %s", line.Device, ctnr.ID())
				continue
			}

			if query.Timestamps {
				builder.WriteString(line.Time.Format(time.RFC3339))
				builder.WriteRune(' ')
			}
			builder.WriteString(line.Msg)

			// Build header and output entry
			binary.BigEndian.PutUint32(header[4:], uint32(len(header)+builder.Len()))
			w.Write(header[:])
			fmt.Fprint(w, builder.String())

			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}

func (s *APIServer) containerExists(w http.ResponseWriter, r *http.Request) {
	// /containers/libpod/{name:..*}/exists
	name := mux.Vars(r)["name"]
	_, err := s.Runtime.LookupContainer(name)
	if err != nil {
		containerNotFound(w, name, err)
		return
	}
	s.WriteResponse(w, http.StatusOK, http.StatusText(http.StatusOK))
}
