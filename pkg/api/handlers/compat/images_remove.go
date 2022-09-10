package compat

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/infra/abi"
	"github.com/containers/storage"
	"github.com/gorilla/schema"
)

func RemoveImage(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		Force   bool `schema:"force"`
		NoPrune bool `schema:"noprune"`
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	if _, found := r.URL.Query()["noprune"]; found {
		if query.NoPrune {
			utils.UnSupportedParameter("noprune")
		}
	}
	name := utils.GetName(r)
	possiblyNormalizedName, err := utils.NormalizeToDockerHub(r, name)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("normalizing image: %w", err))
		return
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}

	options := entities.ImageRemoveOptions{
		Force: query.Force,
	}
	report, rmerrors := imageEngine.Remove(r.Context(), []string{possiblyNormalizedName}, options)
	if len(rmerrors) > 0 && rmerrors[0] != nil {
		err := rmerrors[0]
		if errors.Is(err, storage.ErrImageUnknown) {
			utils.ImageNotFound(w, name, fmt.Errorf("failed to find image %s: %w", name, err))
			return
		}
		if errors.Is(err, storage.ErrImageUsedByContainer) {
			utils.Error(w, http.StatusConflict, fmt.Errorf("image %s is in use: %w", name, err))
			return
		}
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	response := make([]map[string]string, 0, len(report.Untagged)+1)
	for _, d := range report.Deleted {
		deleted := make(map[string]string, 1)
		deleted["Deleted"] = d
		response = append(response, deleted)
	}
	for _, u := range report.Untagged {
		untagged := make(map[string]string, 1)
		untagged["Untagged"] = u
		response = append(response, untagged)
	}
	utils.WriteResponse(w, http.StatusOK, response)
}
