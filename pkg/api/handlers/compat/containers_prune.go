package compat

import (
	"net/http"

	"github.com/containers/podman/v2/libpod"
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

	// Libpod response differs
	if utils.IsLibpodRequest(r) {
		report, err := PruneContainersHelper(w, r, filterFuncs)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}

		utils.WriteResponse(w, http.StatusOK, report)
		return
	}

	report, err := runtime.PruneContainers(filterFuncs)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, report)
}

func PruneContainersHelper(w http.ResponseWriter, r *http.Request, filterFuncs []libpod.ContainerFilter) (
	[]*reports.PruneReport, error) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	reports, err := runtime.PruneContainers(filterFuncs)
	if err != nil {
		utils.InternalServerError(w, err)
		return nil, err
	}
	return reports, nil
}
