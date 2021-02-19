package compat

import (
	"net/http"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/pkg/api/handlers/utils"
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
	newImage, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		utils.ImageNotFound(w, name, errors.Wrapf(err, "failed to find image %s", name))
		return
	}

	results, err := runtime.RemoveImage(r.Context(), newImage, query.Force)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}

	response := make([]map[string]string, 0, len(results.Untagged)+1)
	deleted := make(map[string]string, 1)
	deleted["Deleted"] = results.Deleted
	response = append(response, deleted)

	for _, u := range results.Untagged {
		untagged := make(map[string]string, 1)
		untagged["Untagged"] = u
		response = append(response, untagged)
	}

	utils.WriteResponse(w, http.StatusOK, response)
}
