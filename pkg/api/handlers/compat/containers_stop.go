package compat

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/infra/abi"
	"github.com/gorilla/schema"
)

func StopContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	// Now use the ABI implementation to prevent us from having duplicate
	// code.
	containerEngine := abi.ContainerEngine{Libpod: runtime}

	// /{version}/containers/(name)/stop
	query := struct {
		Ignore        bool `schema:"ignore"`
		DockerTimeout uint `schema:"t"`
		LibpodTimeout uint `schema:"timeout"`
	}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	name := utils.GetName(r)
	options := entities.StopOptions{
		Ignore: query.Ignore,
	}
	if utils.IsLibpodRequest(r) {
		if _, found := r.URL.Query()["timeout"]; found {
			options.Timeout = &query.LibpodTimeout
		}
	} else {
		if _, found := r.URL.Query()["t"]; found {
			options.Timeout = &query.DockerTimeout
		}
	}
	con, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}
	state, err := con.State()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if state == define.ContainerStateStopped || state == define.ContainerStateExited {
		utils.WriteResponse(w, http.StatusNotModified, nil)
		return
	}
	report, err := containerEngine.ContainerStop(r.Context(), []string{name}, options)
	if err != nil {
		if errors.Is(err, define.ErrNoSuchCtr) {
			utils.ContainerNotFound(w, name, err)
			return
		}

		utils.InternalServerError(w, err)
		return
	}

	if len(report) > 0 && report[0].Err != nil {
		utils.InternalServerError(w, report[0].Err)
		return
	}

	// Success
	utils.WriteResponse(w, http.StatusNoContent, nil)
}
