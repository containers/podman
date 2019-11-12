package serviceapi

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"syscall"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/docker/docker/pkg/signal"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

func registerContainersHandlers(r *mux.Router) error {
	r.Handle(unversionedPath("/containers/create"), serviceHandler(createContainer)).Methods("POST")
	r.Handle(unversionedPath("/containers/json"), serviceHandler(listContainers)).Methods("GET")
	r.Handle(unversionedPath("/containers/{name:..*}"), serviceHandler(removeContainer)).Methods("DELETE")
	r.Handle(unversionedPath("/containers/{name:..*}/json"), serviceHandler(container)).Methods("GET")
	r.Handle(unversionedPath("/containers/{name:..*}/kill"), serviceHandler(killContainer)).Methods("POST")
	r.Handle(unversionedPath("/containers/{name:..*}/pause"), serviceHandler(pauseContainer)).Methods("POST")
	r.Handle(unversionedPath("/containers/{name:..*}/rename"), serviceHandler(unsupportedHandler)).Methods("POST")
	r.Handle(unversionedPath("/containers/{name:..*}/restart"), serviceHandler(restartContainer)).Methods("POST")
	r.Handle(unversionedPath("/containers/{name:..*}/start"), serviceHandler(startContainer)).Methods("POST")
	r.Handle(unversionedPath("/containers/{name:..*}/stop"), serviceHandler(stopContainer)).Methods("POST")
	r.Handle(unversionedPath("/containers/{name:..*}/unpause"), serviceHandler(unpauseContainer)).Methods("POST")
	r.Handle(unversionedPath("/containers/{name:..*}/wait"), serviceHandler(waitContainer)).Methods("POST")
	return nil
}

func removeContainer(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	// _X DELETE /{version}/containers/(name)/
	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
	if err != nil {
		Error(w, "no such container", http.StatusNotFound, err)
		return
	}

	ctx := context.Background()
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
		Error(w, "Something went wrong.", http.StatusBadRequest, errors.New("DELETE /containers/{id}?link parameter is not supported."))
		return
	}

	if err := runtime.RemoveContainer(ctx, con, force, vols); err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	w.(ServiceWriter).WriteJSON(http.StatusNoContent, "")
	return
}

func listContainers(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	containers, err := runtime.GetAllContainers()
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}

	infoData, err := runtime.Info()
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
	w.(ServiceWriter).WriteJSON(http.StatusOK, list)
}

func container(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	name := mux.Vars(r)["name"]

	ctnr, err := runtime.LookupContainer(name)
	if err != nil {
		noSuchContainerError(w, name, err)
		return
	}

	infoData, err := runtime.Info()
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "Failed to obtain system memory info"))
		return
	}

	api, err := LibpodToContainer(ctnr, infoData)
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	w.(ServiceWriter).WriteJSON(http.StatusOK, api)
}

func killContainer(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	// /{version}/containers/(name)/kill
	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
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
	w.(ServiceWriter).WriteJSON(http.StatusNoContent, "")
}

func waitContainer(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	// /{version}/containers/(name)/wait
	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
	if err != nil {
		Error(w, fmt.Sprintf("No such container: %s", name), http.StatusNotFound, err)
		return
	}

	exitCode, err := con.Wait()

	var msg string
	if err != nil {
		msg = err.Error()
	}
	w.(ServiceWriter).WriteJSON(http.StatusOK, ContainerWaitOKBody{
		StatusCode: int(exitCode),
		Error: struct {
			Message string
		}{
			Message: msg,
		},
	})
}

func stopContainer(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	// /{version}/containers/(name)/stop
	var (
		stopError error
	)
	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
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
	w.(ServiceWriter).WriteJSON(http.StatusNoContent, "")
}

func pauseContainer(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	// /{version}/containers/(name)/pause
	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
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
	w.(ServiceWriter).WriteJSON(http.StatusNoContent, "")
}

func unpauseContainer(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	// /{version}/containers/(name)/unpause
	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
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
	w.(ServiceWriter).WriteJSON(http.StatusNoContent, "")
}

func startContainer(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	ctx := context.Background()
	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
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
	if err := con.Start(ctx, false); err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	w.(ServiceWriter).WriteJSON(http.StatusNoContent, "")
}

func restartContainer(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	// /{version}/containers/(name)/restart
	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
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

	ctx := context.Background()
	timeout := con.StopTimeout()
	if len(r.Form.Get("t")) > 0 {
		t, err := strconv.Atoi(r.Form.Get("t"))
		if err != nil {
			Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Unable to parse parameter 't': %s", r.Form.Get("t")))
			return
		}
		timeout = uint(t)
	}
	if err := con.RestartWithTimeout(ctx, timeout); err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}

	// Success
	w.(ServiceWriter).WriteJSON(http.StatusNoContent, "")
	return
}
