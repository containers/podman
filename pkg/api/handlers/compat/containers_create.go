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

	// compatible configuration
	body := handlers.CreateContainerConfig{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}

	// Override the container name in the body struct
	body.Name = query.Name

	if len(body.HostConfig.Links) > 0 {
		utils.Error(w, utils.ErrLinkNotSupport.Error(), http.StatusBadRequest, errors.Wrapf(utils.ErrLinkNotSupport, "bad parameter"))
		return
	}
	rtc, err := runtime.GetConfig()
	if err != nil {
		utils.Error(w, "unable to obtain runtime config", http.StatusInternalServerError, errors.Wrap(err, "unable to get runtime config"))
		return
	}

	newImage, err := runtime.ImageRuntime().NewFromLocal(body.Config.Image)
	if err != nil {
		if errors.Cause(err) == define.ErrNoSuchImage {
			utils.Error(w, "No such image", http.StatusNotFound, err)
			return
		}

		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "NewFromLocal()"))
		return
	}

	// Take body structure and convert to cliopts
	cliOpts, args, err := common.ContainerCreateToContainerCLIOpts(body, rtc.Engine.CgroupManager)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "make cli opts()"))
		return
	}

	imgNameOrID := newImage.ID()
	// if the img had multi names with the same sha256 ID, should use the InputName, not the ID
	if len(newImage.Names()) > 1 {
		imageRef, err := utils.ParseDockerReference(newImage.InputName)
		if err != nil {
			utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest, err)
			return
		}
		// maybe the InputName has no tag, so use full name to display
		imgNameOrID = imageRef.DockerReference().String()
	}

	sg := specgen.NewSpecGenerator(imgNameOrID, cliOpts.RootFS)
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
