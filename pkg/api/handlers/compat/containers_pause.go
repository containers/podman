package compat

import (
	"net/http"
	"os"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/api/handlers/utils"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/domain/infra/abi"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func PauseContainer(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	containerEngine := abi.ContainerEngine{Libpod: runtime}
	query := struct {
		All bool `schema:"all"`
	}{}

	err := decoder.Decode(&query, r.URL.Query())
	if err != nil {
		utils.Error(w, "Bad Request.", http.StatusBadRequest, errors.Wrap(err, "couldn't decode the query"))
		return
	}

	// /{version}/containers/(name)/pause
	name := utils.GetName(r)
	options := entities.PauseUnPauseOptions{
		All: query.All,
	}
	reports, err := containerEngine.ContainerPause(r.Context(), []string{name}, options)

	if errors.Cause(err) == define.ErrNoSuchCtr || os.IsNotExist(err) {
		utils.ContainerNotFound(w, name, err)
		return
	}

	for _, r := range reports {
		if r.Err != nil {
			utils.InternalServerError(w, r.Err)
			return
		}
	}

	// Success
	utils.WriteResponse(w, http.StatusNoContent, nil)
}
