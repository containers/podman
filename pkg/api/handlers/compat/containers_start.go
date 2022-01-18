package compat

import (
	"net/http"

	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/sirupsen/logrus"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	"github.com/gorilla/schema"
)

func StartContainer(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
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
		logrus.Info("The detach keys parameter is not supported on start container")
	}
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
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
		utils.WriteResponse(w, http.StatusNotModified, nil)
		return
	}
	if err := con.Start(r.Context(), true); err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, nil)
}
