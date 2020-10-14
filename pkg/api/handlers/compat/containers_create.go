package compat

import (
	"encoding/json"
	"net/http"

	"github.com/containers/podman/v2/cmd/podman/common"
	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/api/handlers"
	"github.com/containers/podman/v2/pkg/api/handlers/utils"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/domain/infra/abi"
	"github.com/containers/podman/v2/pkg/specgen"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func CreateContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	input := handlers.CreateContainerConfig{}
	query := struct {
		Name string `schema:"name"`
	}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}
	if len(input.HostConfig.Links) > 0 {
		utils.Error(w, utils.ErrLinkNotSupport.Error(), http.StatusBadRequest, errors.Wrapf(utils.ErrLinkNotSupport, "bad parameter"))
		return
	}
	newImage, err := runtime.ImageRuntime().NewFromLocal(input.Image)
	if err != nil {
		if errors.Cause(err) == define.ErrNoSuchImage {
			utils.Error(w, "No such image", http.StatusNotFound, err)
			return
		}

		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "NewFromLocal()"))
		return
	}

	// Take input structure and convert to cliopts
	cliOpts, args, err := common.ContainerCreateToContainerCLIOpts(input)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "make cli opts()"))
		return
	}
	sg := specgen.NewSpecGenerator(newImage.ID(), cliOpts.RootFS)
	if err := common.FillOutSpecGen(sg, cliOpts, args); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "fill out specgen"))
		return
	}
	ic := abi.ContainerEngine{Libpod: runtime}
	report, err := ic.ContainerCreate(r.Context(), sg)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "container create"))
		return
	}
	createResponse := entities.ContainerCreateResponse{
		ID:       report.Id,
		Warnings: []string{},
	}
	utils.WriteResponse(w, http.StatusCreated, createResponse)
}
