package utils

import (
	"fmt"
	"net/http"
	"syscall"
	"time"

	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func KillContainer(w http.ResponseWriter, r *http.Request) (*libpod.Container, error) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decorder").(*schema.Decoder)
	query := struct {
		Signal syscall.Signal `schema:"signal"`
	}{
		Signal: syscall.SIGKILL,
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return nil, err
	}
	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
	if err != nil {
		ContainerNotFound(w, name, err)
		return nil, err
	}

	state, err := con.State()
	if err != nil {
		InternalServerError(w, err)
		return con, err
	}

	// If the Container is stopped already, send a 409
	if state == define.ContainerStateStopped || state == define.ContainerStateExited {
		Error(w, fmt.Sprintf("Container %s is not running", name), http.StatusConflict, errors.New(fmt.Sprintf("Cannot kill Container %s, it is not running", name)))
		return con, err
	}

	err = con.Kill(uint(query.Signal))
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "unable to kill Container %s", name))
	}
	return con, err
}

func RemoveContainer(w http.ResponseWriter, r *http.Request, force, vols bool) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
	if err != nil {
		ContainerNotFound(w, name, err)
		return
	}

	if err := runtime.RemoveContainer(r.Context(), con, force, vols); err != nil {
		InternalServerError(w, err)
		return
	}
	WriteResponse(w, http.StatusNoContent, "")
}

func WaitContainer(w http.ResponseWriter, r *http.Request) (int32, error) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	// /{version}/containers/(name)/restart
	query := struct {
		Interval  string `schema:"interval"`
		Condition string `schema:"condition"`
	}{
		// Override golang default values for types
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return 0, err
	}

	if len(query.Condition) > 0 {
		UnSupportedParameter("condition")
	}

	name := mux.Vars(r)["name"]
	con, err := runtime.LookupContainer(name)
	if err != nil {
		ContainerNotFound(w, name, err)
		return 0, err
	}
	if len(query.Interval) > 0 {
		d, err := time.ParseDuration(query.Interval)
		if err != nil {
			Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse %s for interval", query.Interval))
		}
		return con.WaitWithInterval(d)
	}
	return con.Wait()
}

// GenerateFilterFuncsFromMap is used to generate un-executed functions that can be used to filter
// containers.  It is specifically designed for the RESTFUL API input.
func GenerateFilterFuncsFromMap(r *libpod.Runtime, filters map[string][]string) ([]libpod.ContainerFilter, error) {
	var (
		filterFuncs []libpod.ContainerFilter
	)
	for k, v := range filters {
		for _, val := range v {
			f, err := shared.GenerateContainerFilterFuncs(k, val, r)
			if err != nil {
				return filterFuncs, err
			}
			filterFuncs = append(filterFuncs, f)
		}
	}
	return filterFuncs, nil
}
