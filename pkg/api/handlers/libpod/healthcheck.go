package libpod

import (
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/api/handlers/utils"
)

func RunHealthCheck(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := utils.GetName(r)
	status, err := runtime.HealthCheck(name)
	if err != nil {
		if status == define.HealthCheckContainerNotFound {
			utils.ContainerNotFound(w, name, err)
			return
		}
		if status == define.HealthCheckNotDefined {
			utils.Error(w, "no healthcheck defined", http.StatusConflict, err)
			return
		}
		if status == define.HealthCheckContainerStopped {
			utils.Error(w, "container not running", http.StatusConflict, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}
	hcStatus := define.HealthCheckUnhealthy
	if status == define.HealthCheckSuccess {
		hcStatus = define.HealthCheckHealthy
	}
	report := define.HealthCheckResults{
		Status: hcStatus,
	}
	utils.WriteResponse(w, http.StatusOK, report)
}
