package serviceapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"syscall"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/docker/docker/pkg/signal"
	"github.com/gorilla/mux"
)

func registerContainersHandlers(r *mux.Router) error {
	r.Handle(versionedPath("/containers/"), serviceHandler(containers))
	r.Handle(versionedPath("/containers/{name:..*}/json"), serviceHandler(container))
	r.Handle(versionedPath("/containers/{name:..*}/kill"), serviceHandler(killContainer))
	r.Handle(versionedPath("/containers/{name:..*}/pause"), serviceHandler(pauseContainer))
	r.Handle(versionedPath("/containers/{name:..*}/rename"), serviceHandler(renameContainer))
	r.Handle(versionedPath("/containers/{name:..*}/restart"), serviceHandler(restartContainer))
	r.Handle(versionedPath("/containers/{name:..*}/stop"), serviceHandler(stopContainer))
	r.Handle(versionedPath("/containers/{name:..*}/unpause"), serviceHandler(unpauseContainer))
	r.Handle(versionedPath("/containers/{name:..*}/wait"), serviceHandler(waitContainer))
	return nil
}

func containers(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	http.NotFound(w, r)
}

func container(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	// /v1.24/containers/(name)/json
	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
	if err != nil {
		noSuchContainerError(w, name)
		return
	}

	ctx := context.Background()
	switch r.Method {
	case "DELETE":
		var force, vols bool
		if len(r.Form.Get("force")) > 0 {
			force, err = strconv.ParseBool(r.Form.Get("force"))
			if err != nil {
				apiError(w, "unable to parse bool", http.StatusInternalServerError)
			}
		}
		if len(r.Form.Get("b")) > 0 {
			vols, err = strconv.ParseBool(r.Form.Get("v"))
			if err != nil {
				apiError(w, "unable to parse bool", http.StatusInternalServerError)
			}
		}
		if err := runtime.RemoveContainer(ctx, con, force, vols); err != nil {
			apiError(w, fmt.Sprintf("unable to remove %s: %s", name, err.Error()), http.StatusInternalServerError)
			return
		}
		// TODO need to send a 204 on success
		return
	}
	apiError(w, fmt.Sprint("not implemented"), http.StatusInternalServerError)
	return
}

func killContainer(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	// /v1.24/containers/(name)/kill
	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
	if err != nil {
		http.Error(w, fmt.Sprintf("container '%s' not found", name), http.StatusNotFound)
		return
	}

	state, err := con.State()
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to get state for %s : %s", name, err.Error()), http.StatusInternalServerError)
		return
	}

	// If the container is stopped already, send a 409
	if state == define.ContainerStateStopped || state == define.ContainerStateExited {
		containerNotRunningError(w, con.ID())
		return
	}

	sig := syscall.SIGKILL
	if len(r.Form.Get("signal")) > 0 {
		sig, err = signal.ParseSignal(r.Form.Get("signal"))
		if err != nil {
			apiError(w, fmt.Sprintf("unable to parse signal %s: %s", r.Form.Get("signal"), err.Error()), http.StatusInternalServerError)
			return
		}
	}
	if err := con.Kill(uint(sig)); err != nil {
		apiError(w, fmt.Sprintf("unable to kill container %s: %s", name, err.Error()), http.StatusInternalServerError)
		return
	}
	// Success
	w.WriteHeader(http.StatusNoContent)
	fmt.Fprintln(w, "")
	return
}

func waitContainer(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	// /v1.24/containers/(name)/wait
	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
	if err != nil {
		noSuchContainerError(w, name)
		return
	}
	exitCode, err := con.Wait()
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to wait on %s: %s", name, err.Error()), http.StatusInternalServerError)
		return
	}
	// TODO this needs to be formed in a struct for wait
	m := make(map[string]int32)
	m["StatusCode"] = exitCode
	buffer, err := json.Marshal(m)
	if err != nil {
		apiError(w, fmt.Sprintf("unable to marshal reponse  %s", err.Error()), http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, buffer)
}

func stopContainer(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	// /v1.24/containers/(name)/stop
	var (
		stopError error
	)
	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
	if err != nil {
		noSuchContainerError(w, name)
		return
	}

	state, err := con.State()
	if err != nil {
		apiError(w, fmt.Sprintf("unable to get state for %s : %s", name, err.Error()), http.StatusInternalServerError)
		return
	}

	// If the container is stopped already, send a 302
	if state == define.ContainerStateStopped || state == define.ContainerStateExited {
		apiError(w, fmt.Sprintf("container %s is already stopped ", name), http.StatusFound)
		return
	}

	if len(r.Form.Get("t")) > 0 {
		timeout, err := strconv.Atoi(r.Form.Get("t"))
		if err != nil {
			apiError(w, fmt.Sprintf("unable to convert %s to timeout: %s", r.Form.Get("t"), err.Error()), http.StatusInternalServerError)
			return
		}
		stopError = con.StopWithTimeout(uint(timeout))
	} else {
		stopError = con.Stop()
	}
	if stopError != nil {
		apiError(w, fmt.Sprintf("fail to stop %s: %s", name, stopError.Error()), http.StatusInternalServerError)
		return
	}
	// Success
	w.WriteHeader(http.StatusNoContent)
	fmt.Fprintln(w, "")
	return
}

func pauseContainer(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	// /v1.24/containers/(name)/pause
	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
	if err != nil {
		noSuchContainerError(w, name)
		return
	}

	// the api does not error if the container is already paused, so just into it
	if err := con.Pause(); err != nil {
		apiError(w, fmt.Sprintf("unable to pause %s: %s", name, err.Error()), http.StatusInternalServerError)
		return
	}

	// Success
	w.WriteHeader(http.StatusNoContent)
	fmt.Fprintln(w, "")
	return
}

func unpauseContainer(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	// /v1.24/containers/(name)/unpause
	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
	if err != nil {
		noSuchContainerError(w, name)
		return
	}

	// the api does not error if the container is already paused, so just into it
	if err := con.Unpause(); err != nil {
		apiError(w, fmt.Sprintf("unable to unpause %s: %s", name, err.Error()), http.StatusInternalServerError)
		return
	}

	// Success
	w.WriteHeader(http.StatusNoContent)
	fmt.Fprintln(w, "")
	return
}

func restartContainer(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	// /v1.24/containers/(name)/restart
	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
	if err != nil {
		noSuchContainerError(w, name)
		return
	}

	state, err := con.State()
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to get state for %s : %s", name, err.Error()), http.StatusInternalServerError)
		return
	}

	// If the container is stopped already, send a 409
	if state == define.ContainerStateStopped || state == define.ContainerStateExited {
		http.Error(w, fmt.Sprintf("container %s is not running", name), http.StatusConflict)
		return
	}

	ctx := context.Background()
	timeout := con.StopTimeout()
	if len(r.Form.Get("t")) > 0 {
		t, err := strconv.Atoi(r.Form.Get("t"))
		if err != nil {
			apiError(w, fmt.Sprintf("unable to parse timeout %s : %s", r.Form.Get("t"), err.Error()), http.StatusInternalServerError)
			return
		}
		timeout = uint(t)
	}
	if err := con.RestartWithTimeout(ctx, timeout); err != nil {
		apiError(w, fmt.Sprintf("unable to restart %s : %s", name, err.Error()), http.StatusInternalServerError)
		return
	}

	// Success
	w.WriteHeader(http.StatusNoContent)
	fmt.Fprintln(w, "")
	return
}
