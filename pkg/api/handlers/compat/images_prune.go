package compat

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/api/handlers"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/infra/abi"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/docker/docker/api/types"
	"github.com/pkg/errors"
)

func PruneImages(w http.ResponseWriter, r *http.Request) {
	var (
		filters []string
	)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	for k, v := range *filterMap {
		for _, val := range v {
			filters = append(filters, fmt.Sprintf("%s=%s", k, val))
		}
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}

	pruneOptions := entities.ImagePruneOptions{Filter: filters}
	imagePruneReports, err := imageEngine.Prune(r.Context(), pruneOptions)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	idr := make([]types.ImageDeleteResponseItem, len(imagePruneReports))
	var reclaimedSpace uint64
	var errorMsg bytes.Buffer
	for _, p := range imagePruneReports {
		if p.Err != nil {
			// Docker stops on first error vs. libpod which keeps going. Given API constraints, concatenate all errors
			// and return that string.
			errorMsg.WriteString(p.Err.Error())
			errorMsg.WriteString("; ")
			continue
		}

		idr = append(idr, types.ImageDeleteResponseItem{
			Deleted: p.Id,
		})
		reclaimedSpace = reclaimedSpace + p.Size
	}
	if errorMsg.Len() > 0 {
		utils.InternalServerError(w, errors.New(errorMsg.String()))
		return
	}

	payload := handlers.ImagesPruneReport{
		ImagesPruneReport: types.ImagesPruneReport{
			ImagesDeleted:  idr,
			SpaceReclaimed: reclaimedSpace,
		},
	}
	utils.WriteResponse(w, http.StatusOK, payload)
}
