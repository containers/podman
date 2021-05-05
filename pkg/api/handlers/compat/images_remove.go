package compat

import (
	"net/http"

	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/pkg/api/handlers/utils"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/domain/infra/abi"
	"github.com/containers/storage"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func RemoveImage(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	query := struct {
		Force   bool `schema:"force"`
		NoPrune bool `schema:"noprune"`
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	if _, found := r.URL.Query()["noprune"]; found {
		if query.NoPrune {
			utils.UnSupportedParameter("noprune")
		}
	}
	name := utils.GetName(r)
	imageEngine := abi.ImageEngine{Libpod: runtime}

	options := entities.ImageRemoveOptions{
		Force: query.Force,
	}
	report, rmerrors := imageEngine.Remove(r.Context(), []string{name}, options)
	if len(rmerrors) > 0 && rmerrors[0] != nil {
		err := rmerrors[0]
		if errors.Cause(err) == storage.ErrImageUnknown {
			utils.ImageNotFound(w, name, errors.Wrapf(err, "failed to find image %s", name))
			return
		}

		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
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
