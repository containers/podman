package handlers

import (
	"fmt"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"net/http"
)

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

	name := getName(r)
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
	if query.Timeout > 0 {
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

func UnpauseContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	// /{version}/containers/(name)/unpause
	name := getName(r)
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

func PauseContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	// /{version}/containers/(name)/pause
	name := getName(r)
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

func StartContainer(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		DetachKeys string `schema:"detachKeys"`
	}{
		// Override golang default values for types
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	if len(query.DetachKeys) > 0 {
		// TODO - start does not support adding detach keys
		Error(w, "Something went wrong", http.StatusBadRequest, errors.New("the detachKeys parameter is not supported yet"))
		return
	}
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := getName(r)
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

	name := getName(r)
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
