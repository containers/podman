//go:build !remote

package libpod

import (
	"net/http"

	"github.com/containers/podman/v6/libpod"
	"github.com/containers/podman/v6/pkg/api/handlers/utils"
	api "github.com/containers/podman/v6/pkg/api/types"
	"github.com/containers/podman/v6/pkg/domain/infra/abi"
)

func GetInfo(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	containerEngine := abi.ContainerEngine{Libpod: runtime}
	info, err := containerEngine.Info(r.Context())
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, info)
}
