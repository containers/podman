package compat

import (
	"net/http"

	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/pkg/api/handlers/utils"
)

func Changes(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	id := utils.GetName(r)
	changes, err := runtime.GetDiff("", id)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteJSON(w, 200, changes)
}
