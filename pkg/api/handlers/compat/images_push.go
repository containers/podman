package compat

import (
	"context"
	"net/http"
	"strings"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/pkg/api/handlers/utils"
	"github.com/containers/podman/v2/pkg/auth"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/domain/infra/abi"
	"github.com/containers/storage"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

// PushImage is the handler for the compat http endpoint for pushing images.
func PushImage(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	// Now use the ABI implementation to prevent us from having duplicate
	// code.
	imageEngine := abi.ImageEngine{Libpod: runtime}

	query := struct {
		All         bool   `schema:"all"`
		Compress    bool   `schema:"compress"`
		Destination string `schema:"destination"`
		Tag         string `schema:"tag"`
		TLSVerify   bool   `schema:"tlsVerify"`
	}{
		// This is where you can override the golang default value for one of fields
		TLSVerify: true,
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	// Note that Docker's docs state "Image name or ID" to be in the path
	// parameter but it really must be a name as Docker does not allow for
	// pushing an image by ID.
	imageName := strings.TrimSuffix(utils.GetName(r), "/push") // GetName returns the entire path
	if query.Tag != "" {
		imageName += ":" + query.Tag
	}
	if _, err := utils.ParseStorageReference(imageName); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "image source %q is not a containers-storage-transport reference", imageName))
		return
	}

	authconf, authfile, key, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "failed to parse %q header for %s", key, r.URL.String()))
		return
	}
	defer auth.RemoveAuthfile(authfile)
	var username, password string
	if authconf != nil {
		username = authconf.Username
		password = authconf.Password
	}
	options := entities.ImagePushOptions{
		All:      query.All,
		Authfile: authfile,
		Compress: query.Compress,
		Username: username,
		Password: password,
	}
	if err := imageEngine.Push(context.Background(), imageName, query.Destination, options); err != nil {
		if errors.Cause(err) != storage.ErrImageUnknown {
			utils.ImageNotFound(w, imageName, errors.Wrapf(err, "failed to find image %s", imageName))
			return
		}

		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "error pushing image %q", imageName))
		return
	}

	utils.WriteResponse(w, http.StatusOK, "")
}
