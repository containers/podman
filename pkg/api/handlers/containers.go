package handlers

import (
	"fmt"
	"github.com/docker/docker/api/types"
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
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
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	name := getName(r)
	con, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	state, err := con.State()
	if err != nil {
		utils.InternalServerError(w, errors.Wrapf(err, "unable to get state for Container %s", name))
		return
	}
	// If the Container is stopped already, send a 302
	if state == define.ContainerStateStopped || state == define.ContainerStateExited {
		utils.Error(w, http.StatusText(http.StatusNotModified), http.StatusNotModified,
			errors.Errorf("Container %s is already stopped ", name))
		return
	}

	var stopError error
	if query.Timeout > 0 {
		stopError = con.StopWithTimeout(uint(query.Timeout))
	} else {
		stopError = con.Stop()
	}
	if stopError != nil {
		utils.InternalServerError(w, errors.Wrapf(stopError, "failed to stop %s", name))
		return
	}

	// Success
	utils.WriteResponse(w, http.StatusNoContent, "")
}

func UnpauseContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	// /{version}/containers/(name)/unpause
	name := getName(r)
	con, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	// the api does not error if the Container is already paused, so just into it
	if err := con.Unpause(); err != nil {
		utils.InternalServerError(w, err)
		return
	}

	// Success
	utils.WriteResponse(w, http.StatusNoContent, "")
}

func PauseContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	// /{version}/containers/(name)/pause
	name := getName(r)
	con, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	// the api does not error if the Container is already paused, so just into it
	if err := con.Pause(); err != nil {
		utils.InternalServerError(w, err)
		return
	}
	// Success
	utils.WriteResponse(w, http.StatusNoContent, "")
}

func StartContainer(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		DetachKeys string `schema:"detachKeys"`
	}{
		// Override golang default values for types
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	if len(query.DetachKeys) > 0 {
		// TODO - start does not support adding detach keys
		utils.Error(w, "Something went wrong", http.StatusBadRequest, errors.New("the detachKeys parameter is not supported yet"))
		return
	}
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := getName(r)
	con, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	state, err := con.State()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if state == define.ContainerStateRunning {
		msg := fmt.Sprintf("Container %s is already running", name)
		utils.Error(w, msg, http.StatusNotModified, errors.New(msg))
		return
	}
	if err := con.Start(r.Context(), false); err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
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
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	name := getName(r)
	con, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	state, err := con.State()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	// FIXME: This is not in the swagger.yml...
	// If the Container is stopped already, send a 409
	if state == define.ContainerStateStopped || state == define.ContainerStateExited {
		msg := fmt.Sprintf("Container %s is not running", name)
		utils.Error(w, msg, http.StatusConflict, errors.New(msg))
		return
	}

	timeout := con.StopTimeout()
	if _, found := mux.Vars(r)["t"]; found {
		timeout = uint(query.Timeout)
	}

	if err := con.RestartWithTimeout(r.Context(), timeout); err != nil {
		utils.InternalServerError(w, err)
		return
	}

	// Success
	utils.WriteResponse(w, http.StatusNoContent, "")
}

func PruneContainers(w http.ResponseWriter, r *http.Request) {
	var (
		delContainers []string
		space         int64
	)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)

	query := struct {
		Filters map[string][]string `schema:"filter"`
	}{}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	filterFuncs, err := utils.GenerateFilterFuncsFromMap(runtime, query.Filters)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	prunedContainers, pruneErrors, err := runtime.PruneContainers(filterFuncs)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	// Libpod response differs
	if utils.IsLibpodRequest(r) {
		var response []LibpodContainersPruneReport
		for ctrID, size := range prunedContainers {
			response = append(response, LibpodContainersPruneReport{ID: ctrID, SpaceReclaimed: size})
		}
		for ctrID, err := range pruneErrors {
			response = append(response, LibpodContainersPruneReport{ID: ctrID, PruneError: err.Error()})
		}
		utils.WriteResponse(w, http.StatusOK, response)
		return
	}
	for ctrID, size := range prunedContainers {
		if pruneErrors[ctrID] == nil {
			space += size
			delContainers = append(delContainers, ctrID)
		}
	}
	report := types.ContainersPruneReport{
		ContainersDeleted: delContainers,
		SpaceReclaimed:    uint64(space),
	}
	utils.WriteResponse(w, http.StatusOK, report)
}
