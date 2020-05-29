package compat

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/containers/libpod/pkg/auth"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

// PushImage is the handler for the compat http endpoint for pushing images.
func PushImage(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	query := struct {
		Tag string `schema:"tag"`
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
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

	newImage, err := runtime.ImageRuntime().NewFromLocal(imageName)
	if err != nil {
		utils.ImageNotFound(w, imageName, errors.Wrapf(err, "Failed to find image %s", imageName))
		return
	}

	authConf, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse %q header for %s", auth.XRegistryAuthHeader, r.URL.String()))
		return
	}
	defer auth.RemoveAuthfile(authfile)

	dockerRegistryOptions := &image.DockerRegistryOptions{DockerRegistryCreds: authConf}
	if sys := runtime.SystemContext(); sys != nil {
		dockerRegistryOptions.DockerCertPath = sys.DockerCertPath
		dockerRegistryOptions.RegistriesConfPath = sys.SystemRegistriesConfPath
	}

	err = newImage.PushImageToHeuristicDestination(
		context.Background(),
		imageName,
		"", // manifest type
		authfile,
		"", // digest file
		"", // signature policy
		os.Stderr,
		false, // force compression
		image.SigningOptions{},
		dockerRegistryOptions,
		nil, // additional tags
	)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Error pushing image %q", imageName))
		return
	}

	utils.WriteResponse(w, http.StatusOK, "")

}
