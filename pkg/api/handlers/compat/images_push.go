package compat

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/containers/image/v5/types"
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

	digestFile, err := ioutil.TempFile("", "digest.txt")
	if err != nil {
		utils.Error(w, "unable to create digest tempfile", http.StatusInternalServerError, errors.Wrap(err, "unable to create tempfile"))
		return
	}
	defer digestFile.Close()

	// Now use the ABI implementation to prevent us from having duplicate
	// code.
	imageEngine := abi.ImageEngine{Libpod: runtime}

	query := struct {
		All         bool   `schema:"all"`
		Compress    bool   `schema:"compress"`
		Destination string `schema:"destination"`
		Format      string `schema:"format"`
		TLSVerify   bool   `schema:"tlsVerify"`
		Tag         string `schema:"tag"`
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
		All:        query.All,
		Authfile:   authfile,
		Compress:   query.Compress,
		Format:     query.Format,
		Password:   password,
		Username:   username,
		DigestFile: digestFile.Name(),
	}
	if _, found := r.URL.Query()["tlsVerify"]; found {
		options.SkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}

	var destination string
	if _, found := r.URL.Query()["destination"]; found {
		destination = query.Destination
	} else {
		destination = imageName
	}

	if err := imageEngine.Push(r.Context(), imageName, destination, options); err != nil {
		if errors.Cause(err) != storage.ErrImageUnknown {
			utils.ImageNotFound(w, imageName, errors.Wrapf(err, "failed to find image %s", imageName))
			return
		}

		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "error pushing image %q", imageName))
		return
	}

	digestBytes, err := ioutil.ReadAll(digestFile)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to read digest tmp file"))
		return
	}

	tag := query.Tag
	if tag == "" {
		tag = "latest"
	}
	respData := struct {
		Status string `json:"status"`
	}{
		Status: fmt.Sprintf("%s: digest: %s size: null", tag, string(digestBytes)),
	}

	utils.WriteJSON(w, http.StatusOK, &respData)
}
