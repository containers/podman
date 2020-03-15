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
			return
		}
		if status == libpod.HealthCheckNotDefined {
			utils.Error(w, "no healthcheck defined", http.StatusConflict, err)
			return
		}
		if status == libpod.HealthCheckContainerStopped {
			utils.Error(w, "container not running", http.StatusConflict, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}
	ctr, err := runtime.LookupContainer(name)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	hcLog, err := ctr.GetHealthCheckLog()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	utils.WriteResponse(w, http.StatusOK, hcLog)
}
