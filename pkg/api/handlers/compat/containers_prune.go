package compat

import (
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/docker/docker/api/types"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func PruneContainers(w http.ResponseWriter, r *http.Request) {
	var (
		delContainers []string
		space         int64
	)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)

	query := struct {
		Filters map[string][]string `schema:"filters"`
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
		var response []handlers.LibpodContainersPruneReport
		for ctrID, size := range prunedContainers {
			response = append(response, handlers.LibpodContainersPruneReport{ID: ctrID, SpaceReclaimed: size})
		}
		for ctrID, err := range pruneErrors {
			response = append(response, handlers.LibpodContainersPruneReport{ID: ctrID, PruneError: err.Error()})
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
