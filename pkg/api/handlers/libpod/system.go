//go:build !remote

package libpod

import (
	"fmt"
	"net/http"
	"time"

	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/pkg/api/handlers/utils"
	api "github.com/containers/podman/v5/pkg/api/types"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/domain/infra/abi"
	"github.com/containers/podman/v5/pkg/util"
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

func SystemCheck(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		Quick                       bool   `schema:"quick"`
		Repair                      bool   `schema:"repair"`
		RepairLossy                 bool   `schema:"repair_lossy"`
		UnreferencedLayerMaximumAge string `schema:"unreferenced_layer_max_age"`
	}{}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest,
			fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}

	var unreferencedLayerMaximumAge *time.Duration
	if query.UnreferencedLayerMaximumAge != "" {
		duration, err := time.ParseDuration(query.UnreferencedLayerMaximumAge)
		if err != nil {
			utils.Error(w, http.StatusBadRequest,
				fmt.Errorf("failed to parse unreferenced_layer_max_age parameter %q for %s: %w", query.UnreferencedLayerMaximumAge, r.URL.String(), err))
		}
		unreferencedLayerMaximumAge = &duration
	}
	checkOptions := entities.SystemCheckOptions{
		Quick:                       query.Quick,
		Repair:                      query.Repair,
		RepairLossy:                 query.RepairLossy,
		UnreferencedLayerMaximumAge: unreferencedLayerMaximumAge,
	}
	report, err := containerEngine.SystemCheck(r.Context(), checkOptions)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	utils.WriteResponse(w, http.StatusOK, report)
}
