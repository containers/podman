package compat

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/pkg/api/handlers"
	"github.com/containers/podman/v2/pkg/api/handlers/utils"
	"github.com/docker/docker/api/types"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func PruneImages(w http.ResponseWriter, r *http.Request) {
	var (
		filters []string
	)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	query := struct {
		All     bool
		Filters map[string][]string `schema:"filters"`
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	for k, v := range query.Filters {
		for _, val := range v {
			filters = append(filters, fmt.Sprintf("%s=%s", k, val))
		}
	}
	imagePruneReports, err := runtime.ImageRuntime().PruneImages(r.Context(), query.All, filters)
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
