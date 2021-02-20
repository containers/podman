package libpod

import (
	"net/http"

	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/pkg/api/handlers/utils"
)

func GetInfo(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	info, err := runtime.Info()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, info)
}
