package compat

import (
	"fmt"
	"net/http"

	"github.com/containers/common/libimage"
	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/pkg/api/handlers/utils"
	"github.com/pkg/errors"
)

func TagImage(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	// /v1.xx/images/(name)/tag
	name := utils.GetName(r)

	lookupOptions := &libimage.LookupImageOptions{IgnorePlatform: true}
	newImage, _, err := runtime.LibimageRuntime().LookupImage(name, lookupOptions)
	if err != nil {
		utils.ImageNotFound(w, name, errors.Wrapf(err, "failed to find image %s", name))
		return
	}

	tag := "latest"
	if len(r.Form.Get("tag")) > 0 {
		tag = r.Form.Get("tag")
	}
	if len(r.Form.Get("repo")) < 1 {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.New("repo parameter is required to tag an image"))
		return
	}
	repo := r.Form.Get("repo")
	tagName := fmt.Sprintf("%s:%s", repo, tag)
	if err := newImage.Tag(tagName); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusCreated, "")
}
