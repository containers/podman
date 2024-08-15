//go:build !remote

package compat

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"

	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/pkg/api/handlers"
	"github.com/containers/podman/v5/pkg/api/handlers/utils"
	api "github.com/containers/podman/v5/pkg/api/types"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/domain/infra/abi"
	"github.com/containers/podman/v5/pkg/util"
	dockerImage "github.com/docker/docker/api/types/image"
)

func PruneImages(w http.ResponseWriter, r *http.Request) {
	var filters []string
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
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

	idr := make([]dockerImage.DeleteResponse, 0, len(imagePruneReports))
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

		idr = append(idr, dockerImage.DeleteResponse{
			Deleted: p.Id,
		})
		reclaimedSpace += p.Size
	}
	if errorMsg.Len() > 0 {
		utils.InternalServerError(w, errors.New(errorMsg.String()))
		return
	}

	payload := handlers.ImagesPruneReport{
		ImagesPruneReport: dockerImage.PruneReport{
			ImagesDeleted:  idr,
			SpaceReclaimed: reclaimedSpace,
		},
	}
	utils.WriteResponse(w, http.StatusOK, payload)
}
