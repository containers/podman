package libpod

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/pkg/api/handlers/utils"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/specgen"
	"github.com/containers/podman/v3/pkg/specgen/generate"
	"github.com/pkg/errors"
)

// CreateContainer takes a specgenerator and makes a container. It returns
// the new container ID on success along with any warnings.
func CreateContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	var sg specgen.SpecGenerator
	if err := json.NewDecoder(r.Body).Decode(&sg); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}
	warn, err := generate.CompleteSpec(r.Context(), runtime, &sg)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	ctr, err := generate.MakeContainer(context.Background(), runtime, &sg)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	response := entities.ContainerCreateResponse{ID: ctr.ID(), Warnings: warn}
	utils.WriteJSON(w, http.StatusCreated, response)
}
