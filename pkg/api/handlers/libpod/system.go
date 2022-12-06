package libpod

import (
	"fmt"
	"net/http"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/infra/abi"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/gorilla/schema"
)

// SystemPrune removes unused data
func SystemPrune(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		All      bool `schema:"all"`
		Volumes  bool `schema:"volumes"`
		External bool `schema:"external"`
	}{}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest,
			fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest,
			fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}

	pruneOptions := entities.SystemPruneOptions{
		All:      query.All,
		Volume:   query.Volumes,
		Filters:  *filterMap,
		External: query.External,
	}
	report, err := containerEngine.SystemPrune(r.Context(), pruneOptions)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	utils.WriteResponse(w, http.StatusOK, report)
}

func DiskUsage(w http.ResponseWriter, r *http.Request) {
	// Options are only used by the CLI
	options := entities.SystemDfOptions{}
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	ic := abi.ContainerEngine{Libpod: runtime}
	response, err := ic.SystemDf(r.Context(), options)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, response)
}
