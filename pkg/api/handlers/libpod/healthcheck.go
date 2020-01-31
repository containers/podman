package libpod

import (
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/api/handlers/utils"
)

func RunHealthCheck(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := utils.GetName(r)
	status, err := runtime.HealthCheck(name)
	if err != nil {
		if status == libpod.HealthCheckContainerNotFound {
			utils.ContainerNotFound(w, name, err)
		}
		utils.InternalServerError(w, err)
	}
	utils.WriteResponse(w, http.StatusOK, status)
}
