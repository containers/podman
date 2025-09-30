//go:build !remote

package libpod

import (
	"fmt"
	"net/http"

	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/pkg/api/handlers/utils"
	api "github.com/containers/podman/v5/pkg/api/types"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/domain/infra/abi"
	"github.com/containers/podman/v5/pkg/util"
)

func ListQuadlets(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	filterMap, err := util.FiltersFromRequest(r)
	if err != nil {
		utils.Error(
			w, http.StatusInternalServerError,
			fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err),
		)
		return
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}

	quadlets, err := containerEngine.QuadletList(r.Context(), entities.QuadletListOptions{Filters: filterMap})
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	utils.WriteResponse(w, http.StatusOK, quadlets)
}
