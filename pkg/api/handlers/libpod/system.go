package libpod

import (
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/api/handlers/compat"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

// SystemPrune removes unused data
func SystemPrune(w http.ResponseWriter, r *http.Request) {
	var (
		decoder           = r.Context().Value("decoder").(*schema.Decoder)
		runtime           = r.Context().Value("runtime").(*libpod.Runtime)
		systemPruneReport = new(entities.SystemPruneReport)
	)
	query := struct {
		All     bool `schema:"all"`
		Volumes bool `schema:"volumes"`
	}{}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	podPruneReport, err := PodPruneHelper(w, r)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	systemPruneReport.PodPruneReport = podPruneReport

	// We could parallelize this, should we?
	containerPruneReport, err := compat.PruneContainersHelper(w, r, nil)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	systemPruneReport.ContainerPruneReport = containerPruneReport

	results, err := runtime.ImageRuntime().PruneImages(r.Context(), query.All, nil)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	report := entities.ImagePruneReport{
		Report: entities.Report{
			Id:  results,
			Err: nil,
		},
	}

	systemPruneReport.ImagePruneReport = &report

	if query.Volumes {
		volumePruneReport, err := pruneVolumesHelper(w, r)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		systemPruneReport.VolumePruneReport = volumePruneReport
	}
	utils.WriteResponse(w, http.StatusOK, systemPruneReport)
}

// SystemReset Resets podman storage back to default state
func SystemReset(w http.ResponseWriter, r *http.Request) {
	err := r.Context().Value("runtime").(*libpod.Runtime).Reset(r.Context())
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, nil)
}
