package compat

import (
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func StartContainer(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		DetachKeys string `schema:"detachKeys"`
	}{
		// Override golang default values for types
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.BadRequest(w, "url", r.URL.String(), err)
		return
	}
	if len(query.DetachKeys) > 0 {
		// TODO - start does not support adding detach keys
		utils.BadRequest(w, "detachKeys", query.DetachKeys, errors.New("the detachKeys parameter is not supported yet"))
		return
	}
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := utils.GetName(r)
	con, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	state, err := con.State()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if state == define.ContainerStateRunning {
		utils.WriteResponse(w, http.StatusNotModified, "")
		return
	}
	if err := con.Start(r.Context(), false); err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}
