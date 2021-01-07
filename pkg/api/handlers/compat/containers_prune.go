package compat

import (
	"bytes"
	"net/http"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/pkg/api/handlers"
	"github.com/containers/podman/v2/pkg/api/handlers/utils"
	"github.com/containers/podman/v2/pkg/domain/entities/reports"
	"github.com/containers/podman/v2/pkg/domain/filters"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func PruneContainers(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)

	query := struct {
		Filters map[string][]string `schema:"filters"`
	}{}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	filterFuncs := make([]libpod.ContainerFilter, 0, len(query.Filters))
	for k, v := range query.Filters {
		generatedFunc, err := filters.GenerateContainerFilterFuncs(k, v, runtime)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		filterFuncs = append(filterFuncs, generatedFunc)
	}

	report, err := PruneContainersHelper(r, filterFuncs)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	// Libpod response differs
	if utils.IsLibpodRequest(r) {
		utils.WriteResponse(w, http.StatusOK, report)
		return
	}

	var payload handlers.ContainersPruneReport
	var errorMsg bytes.Buffer
	for _, pr := range report {
		if pr.Err != nil {
			// Docker stops on first error vs. libpod which keeps going. Given API constraints, concatenate all errors
			// and return that string.
			errorMsg.WriteString(pr.Err.Error())
			errorMsg.WriteString("; ")
			continue
		}
		payload.ContainersDeleted = append(payload.ContainersDeleted, pr.Id)
		payload.SpaceReclaimed += pr.Size
	}
	if errorMsg.Len() > 0 {
		utils.InternalServerError(w, errors.New(errorMsg.String()))
		return
	}

	utils.WriteResponse(w, http.StatusOK, payload)
}

func PruneContainersHelper(r *http.Request, filterFuncs []libpod.ContainerFilter) ([]*reports.PruneReport, error) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	report, err := runtime.PruneContainers(filterFuncs)
	if err != nil {
		return nil, err
	}
	return report, nil
}
