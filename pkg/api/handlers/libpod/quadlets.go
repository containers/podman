//go:build !remote

package libpod

import (
	"fmt"
	"net/http"

	"github.com/containers/podman/v6/libpod"
	"github.com/containers/podman/v6/pkg/api/handlers/utils"
	api "github.com/containers/podman/v6/pkg/api/types"
	"github.com/containers/podman/v6/pkg/domain/entities"
	"github.com/containers/podman/v6/pkg/domain/infra/abi"
	"github.com/containers/podman/v6/pkg/util"
	"github.com/sirupsen/logrus"
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

func GetQuadletPrint(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)

	containerEngine := abi.ContainerEngine{Libpod: runtime}

	quadletContents, err := containerEngine.QuadletPrint(r.Context(), name)
	if err != nil {
		utils.Error(w, http.StatusNotFound, fmt.Errorf("no such quadlet: %s: %w", name, err))
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(quadletContents)); err != nil {
		logrus.Errorf("Failed to write quadlet contents: %v", err)
		return
	}
}
