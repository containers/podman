package compat

import (
	"encoding/json"
	"net/http"

	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/api/handlers"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/infra/abi"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/containers/podman/v4/pkg/specgenutil"
	"github.com/containers/storage"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func CreateContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Name string `schema:"name"`
	}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	// compatible configuration
	body := handlers.CreateContainerConfig{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}

	// Override the container name in the body struct
	body.Name = query.Name

	if len(body.HostConfig.Links) > 0 {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(utils.ErrLinkNotSupport, "bad parameter"))
		return
	}
	rtc, err := runtime.GetConfig()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "unable to get runtime config"))
		return
	}

	imageName, err := utils.NormalizeToDockerHub(r, body.Config.Image)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "error normalizing image"))
		return
	}
	body.Config.Image = imageName

	newImage, resolvedName, err := runtime.LibimageRuntime().LookupImage(body.Config.Image, nil)
	if err != nil {
		if errors.Cause(err) == storage.ErrImageUnknown {
			utils.Error(w, http.StatusNotFound, errors.Wrap(err, "No such image"))
			return
		}

		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "error looking up image"))
		return
	}

	// Take body structure and convert to cliopts
	cliOpts, args, err := common.ContainerCreateToContainerCLIOpts(body, rtc)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "make cli opts()"))
		return
	}

	imgNameOrID := newImage.ID()
	// if the img had multi names with the same sha256 ID, should use the InputName, not the ID
	if len(newImage.Names()) > 1 {
		if err := utils.IsRegistryReference(resolvedName); err != nil {
			utils.Error(w, http.StatusBadRequest, err)
			return
		}
		// maybe the InputName has no tag, so use full name to display
		imgNameOrID = resolvedName
	}

	sg := specgen.NewSpecGenerator(imgNameOrID, cliOpts.RootFS)
	if err := specgenutil.FillOutSpecGen(sg, cliOpts, args); err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "fill out specgen"))
		return
	}
	// moby always create the working directory
	sg.CreateWorkingDir = true

	ic := abi.ContainerEngine{Libpod: runtime}
	report, err := ic.ContainerCreate(r.Context(), sg)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "container create"))
		return
	}
	createResponse := entities.ContainerCreateResponse{
		ID:       report.Id,
		Warnings: []string{},
	}
	utils.WriteResponse(w, http.StatusCreated, createResponse)
}
