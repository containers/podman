package compat

import (
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/api/handlers/utils"
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
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
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
		utils.ImageNotFound(w, name, errors.Wrapf(err, "Failed to find image %s", name))
		return
	}

	_, err = runtime.RemoveImage(r.Context(), newImage, query.Force)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	// TODO
	// This will need to be fixed for proper response, like Deleted: and Untagged:
	m := make(map[string]string)
	m["Deleted"] = newImage.ID()
	foo := []map[string]string{}
	foo = append(foo, m)
	utils.WriteResponse(w, http.StatusOK, foo)

}
