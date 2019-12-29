package libpod

import (
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/gorilla/mux"
)

func RunHealthCheck(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := mux.Vars(r)["name"]
	status, err := runtime.HealthCheck(name)
	if err != nil {
		if status == libpod.HealthCheckContainerNotFound {
			utils.ContainerNotFound(w, name, err)
		}
		utils.InternalServerError(w, err)
	}
	utils.WriteResponse(w, http.StatusOK, status)
}
