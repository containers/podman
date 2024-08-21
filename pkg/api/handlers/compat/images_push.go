//go:build !remote

package compat

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/pkg/api/handlers/utils"
	api "github.com/containers/podman/v5/pkg/api/types"
	"github.com/containers/podman/v5/pkg/auth"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/domain/infra/abi"
	"github.com/containers/storage"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/sirupsen/logrus"
)

// PushImage is the handler for the compat http endpoint for pushing images.
func PushImage(w http.ResponseWriter, r *http.Request) {
	decoder := utils.GetDecoder(r)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

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
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
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
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("image source %q is not a containers-storage-transport reference: %w", imageName, err))
		return
	}

	possiblyNormalizedName, err := utils.NormalizeToDockerHub(r, imageName)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("normalizing image: %w", err))
		return
	}
	imageName = possiblyNormalizedName
	localImage, _, err := runtime.LibimageRuntime().LookupImage(possiblyNormalizedName, nil)
	if err != nil {
		utils.ImageNotFound(w, imageName, fmt.Errorf("failed to find image %s: %w", imageName, err))
		return
	}
	rawManifest, _, err := localImage.Manifest(r.Context())
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	authconf, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
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
		Format:   query.Format,
		Password: password,
		Username: username,
		Quiet:    true,
		Progress: make(chan types.ProgressProperties),
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

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)

	flush := func() {}
	if flusher, ok := w.(http.Flusher); ok {
		flush = flusher.Flush
	}

	statusWritten := false
	writeStatusCode := func(code int) {
		if !statusWritten {
			w.WriteHeader(code)
			w.Header().Set("Content-Type", "application/json")
			flush()
			statusWritten = true
		}
	}

	referenceWritten := false
	writeReference := func() {
		if !referenceWritten {
			var report jsonmessage.JSONMessage
			report.Status = fmt.Sprintf("The push refers to repository [%s]", imageName)
			if err := enc.Encode(report); err != nil {
				logrus.Warnf("Failed to json encode error %q", err.Error())
			}
			flush()
			referenceWritten = true
		}
	}

	pushErrChan := make(chan error)
	var pushReport *entities.ImagePushReport
	go func() {
		var err error
		pushReport, err = imageEngine.Push(r.Context(), imageName, destination, options)
		pushErrChan <- err
	}()

loop: // break out of for/select infinite loop
	for {
		var report jsonmessage.JSONMessage

		select {
		case e := <-options.Progress:
			writeStatusCode(http.StatusOK)
			writeReference()
			switch e.Event {
			case types.ProgressEventNewArtifact:
				report.Status = "Preparing"
			case types.ProgressEventRead:
				report.Status = "Pushing"
				report.Progress = &jsonmessage.JSONProgress{
					Current: int64(e.Offset),
					Total:   e.Artifact.Size,
				}
				report.ProgressMessage = report.Progress.String()
			case types.ProgressEventSkipped:
				report.Status = "Layer already exists"
			case types.ProgressEventDone:
				report.Status = "Pushed"
			}
			report.ID = e.Artifact.Digest.Encoded()[0:12]
			if err := enc.Encode(report); err != nil {
				logrus.Warnf("Failed to json encode error %q", err.Error())
			}
			flush()
		case err := <-pushErrChan:
			if err != nil {
				var msg string
				if errors.Is(err, storage.ErrImageUnknown) {
					// Image may have been removed in the meantime.
					writeStatusCode(http.StatusNotFound)
					msg = "An image does not exist locally with the tag: " + imageName
				} else {
					writeStatusCode(http.StatusInternalServerError)
					msg = err.Error()
				}
				report.Error = &jsonmessage.JSONError{
					Message: msg,
				}
				report.ErrorMessage = msg
				if err := enc.Encode(report); err != nil {
					logrus.Warnf("Failed to json encode error %q", err.Error())
				}
				flush()
				break loop
			} else {
				writeStatusCode(http.StatusOK)
				writeReference() // There may not be any progress, so make sure the reference gets written
			}

			tag := query.Tag
			if tag == "" {
				tag = "latest"
			}

			var digestStr string
			if pushReport != nil {
				digestStr = pushReport.ManifestDigest
			}
			report.Status = fmt.Sprintf("%s: digest: %s size: %d", tag, digestStr, len(rawManifest))
			if err := enc.Encode(report); err != nil {
				logrus.Warnf("Failed to json encode error %q", err.Error())
			}

			flush()
			break loop // break out of for/select infinite loop
		}
	}
}
