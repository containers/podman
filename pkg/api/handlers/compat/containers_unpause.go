package compat

import (
	"net/http"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
)

func UnpauseContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	// /{version}/containers/(name)/unpause
	name := utils.GetName(r)
	con, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	if err := con.Unpause(); err != nil {
		utils.InternalServerError(w, err)
		return
	}

	// Success
	utils.WriteResponse(w, http.StatusNoContent, nil)
}
