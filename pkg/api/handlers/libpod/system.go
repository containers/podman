package libpod

import (
	"net/http"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/pkg/api/handlers/compat"
	"github.com/containers/podman/v2/pkg/api/handlers/utils"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/domain/infra/abi"
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
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	podPruneReport, err := PodPruneHelper(r)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	systemPruneReport.PodPruneReport = podPruneReport

	// We could parallelize this, should we?
	containerPruneReports, err := compat.PruneContainersHelper(r, nil)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	systemPruneReport.ContainerPruneReports = containerPruneReports

	imagePruneReports, err := runtime.ImageRuntime().PruneImages(r.Context(), query.All, nil)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	systemPruneReport.ImagePruneReports = imagePruneReports

	if query.Volumes {
		volumePruneReports, err := pruneVolumesHelper(r)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		systemPruneReport.VolumePruneReports = volumePruneReports
	}
	utils.WriteResponse(w, http.StatusOK, systemPruneReport)
}

func DiskUsage(w http.ResponseWriter, r *http.Request) {
	// Options are only used by the CLI
	options := entities.SystemDfOptions{}
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	ic := abi.ContainerEngine{Libpod: runtime}
	response, err := ic.SystemDf(r.Context(), options)
	if err != nil {
		utils.InternalServerError(w, err)
	}
	utils.WriteResponse(w, http.StatusOK, response)
}
