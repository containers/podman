package serviceapi

import (
	"fmt"
	"net/http"
	"strconv"
	"syscall"

	"github.com/containers/libpod/libpod/define"
	"github.com/docker/docker/pkg/signal"
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
	r.HandleFunc(versionedPath("/containers/{name:..*}/pause"), s.serviceHandler(s.pauseContainer)).Methods("POST")
	r.HandleFunc(versionedPath("/containers/{name:..*}/rename"), s.serviceHandler(s.unsupportedHandler)).Methods("POST")
	r.HandleFunc(versionedPath("/containers/{name:..*}/restart"), s.serviceHandler(s.restartContainer)).Methods("POST")
	r.HandleFunc(versionedPath("/containers/{name:..*}/start"), s.serviceHandler(s.startContainer)).Methods("POST")
	r.HandleFunc(versionedPath("/containers/{name:..*}/stop"), s.serviceHandler(s.stopContainer)).Methods("POST")
	r.HandleFunc(versionedPath("/containers/{name:..*}/unpause"), s.serviceHandler(s.unpauseContainer)).Methods("POST")
	r.HandleFunc(versionedPath("/containers/{name:..*}/wait"), s.serviceHandler(s.waitContainer)).Methods("POST")
	return nil
}

func (s *APIServer) removeContainer(w http.ResponseWriter, r *http.Request) {
	// _X DELETE /{version}/containers/(name)/
	name := mux.Vars(r)["name"]
	con, err := s.Runtime.LookupContainer(name)
	if err != nil {
		Error(w, "no such container", http.StatusNotFound, err)
		return
	}

	var force, vols bool
	if len(r.Form.Get("force")) > 0 {
		force, err = strconv.ParseBool(r.Form.Get("force"))
		if err != nil {
			Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Unable to parse parameter 'force': %s", r.Form.Get("force")))
			return
		}
	}
	if len(r.Form.Get("v")) > 0 {
		vols, err = strconv.ParseBool(r.Form.Get("v"))
		if err != nil {
			Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Unable to parse parameter 'v': %s", r.Form.Get("v")))
			return
		}
	}
	if len(r.Form.Get("link")) > 0 {
		log.Infof("DELETE /containers/{%s}?link parameter is not supported", name)
	}

	if err := s.Runtime.RemoveContainer(s.Context, con, force, vols); err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	s.WriteResponse(w, http.StatusNoContent, "")
}

func (s *APIServer) listContainers(w http.ResponseWriter, r *http.Request) {
	containers, err := s.Runtime.GetAllContainers()
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}

	infoData, err := s.Runtime.Info()
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "Failed to obtain system memory info"))
		return
	}

	var list = make([]*Container, len(containers))
	for i, ctnr := range containers {
		api, err := LibpodToContainer(ctnr, infoData)
		if err != nil {
			Error(w, "Something went wrong.", http.StatusInternalServerError, err)
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
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "Failed to obtain system memory info"))
		return
	}

	api, err := LibpodToContainer(ctnr, infoData)
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	s.WriteResponse(w, http.StatusOK, api)
}

func (s *APIServer) killContainer(w http.ResponseWriter, r *http.Request) {
	// /{version}/containers/(name)/kill
	name := mux.Vars(r)["name"]
	con, err := s.Runtime.LookupContainer(name)
	if err != nil {
		Error(w, fmt.Sprintf("No such container: %s", name), http.StatusNotFound, err)
		return
	}

	state, err := con.State()
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}

	// If the container is stopped already, send a 409
	if state == define.ContainerStateStopped || state == define.ContainerStateExited {
		Error(w, fmt.Sprintf("Container %s is not running", name), http.StatusConflict, errors.New(fmt.Sprintf("Cannot kill container %s, it is not running", name)))
		return
	}

	sig := syscall.SIGKILL
	if len(r.Form.Get("signal")) > 0 {
		sig, err = signal.ParseSignal(r.Form.Get("signal"))
		if err != nil {
			Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "unable to parse signal %s", r.Form.Get("signal")))
			return
		}
	}
	if err := con.Kill(uint(sig)); err != nil {
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
		Error(w, fmt.Sprintf("No such container: %s", name), http.StatusNotFound, err)
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
	var (
		stopError error
	)
	name := mux.Vars(r)["name"]
	con, err := s.Runtime.LookupContainer(name)
	if err != nil {
		Error(w, fmt.Sprintf("No such container: %s", name), http.StatusNotFound, err)
		return
	}

	state, err := con.State()
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, fmt.Sprintf("unable to get state for %s : %s", name)))
		return
	}

	// If the container is stopped already, send a 302
	if state == define.ContainerStateStopped || state == define.ContainerStateExited {
		Error(w, "Something went wrong.", http.StatusNotModified, errors.Wrapf(err, fmt.Sprintf("container %s is already stopped ", name)))
		return
	}

	if len(r.Form.Get("t")) > 0 {
		timeout, err := strconv.Atoi(r.Form.Get("t"))
		if err != nil {
			Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Unable to parse parameter 't': %s", r.Form.Get("t")))
			return
		}
		stopError = con.StopWithTimeout(uint(timeout))
	} else {
		stopError = con.Stop()
	}
	if stopError != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, fmt.Sprintf("failed to stop %s", name)))
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
		Error(w, fmt.Sprintf("No such container: %s", name), http.StatusNotFound, err)
		return
	}

	// the api does not error if the container is already paused, so just into it
	if err := con.Pause(); err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
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
		Error(w, fmt.Sprintf("No such container: %s", name), http.StatusNotFound, err)
		return
	}

	// the api does not error if the container is already paused, so just into it
	if err := con.Unpause(); err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}

	// Success
	s.WriteResponse(w, http.StatusNoContent, "")
}

func (s *APIServer) startContainer(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	con, err := s.Runtime.LookupContainer(name)
	if err != nil {
		Error(w, fmt.Sprintf("No such container: %s", name), http.StatusNotFound, err)
		return
	}
	state, err := con.State()
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	if state == define.ContainerStateRunning {
		msg := fmt.Sprintf("Container %s is already running", name)
		Error(w, msg, http.StatusNotModified, errors.New(msg))
		return
	}
	if err := con.Start(s.Context, false); err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	s.WriteResponse(w, http.StatusNoContent, "")
}

func (s *APIServer) restartContainer(w http.ResponseWriter, r *http.Request) {
	// /{version}/containers/(name)/restart
	name := mux.Vars(r)["name"]
	con, err := s.Runtime.LookupContainer(name)
	if err != nil {
		Error(w, fmt.Sprintf("No such container: %s", name), http.StatusNotFound, err)
		return
	}

	state, err := con.State()
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
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
	if len(r.Form.Get("t")) > 0 {
		t, err := strconv.Atoi(r.Form.Get("t"))
		if err != nil {
			Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Unable to parse parameter 't': %s", r.Form.Get("t")))
			return
		}
		timeout = uint(t)
	}
	if err := con.RestartWithTimeout(s.Context, timeout); err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}

	// Success
	s.WriteResponse(w, http.StatusNoContent, "")
}
