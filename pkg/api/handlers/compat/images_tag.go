package compat

import (
	"fmt"
	"net/http"

	"github.com/containers/common/libimage"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/pkg/errors"
)

func TagImage(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	name := utils.GetName(r)
	possiblyNormalizedName, err := utils.NormalizeToDockerHub(r, name)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "error normalizing image"))
		return
	}

	// Allow tagging manifest list instead of resolving instances from manifest
	lookupOptions := &libimage.LookupImageOptions{ManifestList: true}
	newImage, _, err := runtime.LibimageRuntime().LookupImage(possiblyNormalizedName, lookupOptions)
	if err != nil {
		utils.ImageNotFound(w, name, errors.Wrapf(err, "failed to find image %s", name))
		return
	}

	tag := "latest"
	if len(r.Form.Get("tag")) > 0 {
		tag = r.Form.Get("tag")
	}
	if len(r.Form.Get("repo")) < 1 {
		utils.Error(w, http.StatusBadRequest, errors.New("repo parameter is required to tag an image"))
		return
	}
	repo := r.Form.Get("repo")
	tagName := fmt.Sprintf("%s:%s", repo, tag)

	possiblyNormalizedTag, err := utils.NormalizeToDockerHub(r, tagName)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "error normalizing image"))
		return
	}

	if err := newImage.Tag(possiblyNormalizedTag); err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusCreated, "")
}
