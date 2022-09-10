package libpod

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/auth"
	"github.com/containers/podman/v4/pkg/channel"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/infra/abi"
	"github.com/gorilla/schema"
	"github.com/sirupsen/logrus"
)

// PushImage is the handler for the compat http endpoint for pushing images.
func PushImage(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		All               bool   `schema:"all"`
		CompressionFormat string `schema:"compressionFormat"`
		Destination       string `schema:"destination"`
		Format            string `schema:"format"`
		RemoveSignatures  bool   `schema:"removeSignatures"`
		TLSVerify         bool   `schema:"tlsVerify"`
		Quiet             bool   `schema:"quiet"`
	}{
		TLSVerify: true,
		// #14971: older versions did not sent *any* data, so we need
		//         to be quiet by default to remain backwards compatible
		Quiet: true,
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	source := strings.TrimSuffix(utils.GetName(r), "/push") // GetName returns the entire path
	if _, err := utils.ParseStorageReference(source); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	destination := query.Destination
	if destination == "" {
		destination = source
	}

	if err := utils.IsRegistryReference(destination); err != nil {
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
		All:               query.All,
		Authfile:          authfile,
		CompressionFormat: query.CompressionFormat,
		Format:            query.Format,
		Password:          password,
		Quiet:             true,
		RemoveSignatures:  query.RemoveSignatures,
		Username:          username,
	}

	if _, found := r.URL.Query()["tlsVerify"]; found {
		options.SkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}

	// Let's keep thing simple when running in quiet mode and push directly.
	if query.Quiet {
		if err := imageEngine.Push(r.Context(), source, destination, options); err != nil {
			utils.Error(w, http.StatusBadRequest, fmt.Errorf("pushing image %q: %w", destination, err))
			return
		}
		utils.WriteResponse(w, http.StatusOK, "")
		return
	}

	writer := channel.NewWriter(make(chan []byte))
	defer writer.Close()
	options.Writer = writer

	pushCtx, pushCancel := context.WithCancel(r.Context())
	var pushError error
	go func() {
		defer pushCancel()
		pushError = imageEngine.Push(pushCtx, source, destination, options)
	}()

	flush := func() {
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	flush()

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	for {
		var report entities.ImagePushReport
		select {
		case s := <-writer.Chan():
			report.Stream = string(s)
			if err := enc.Encode(report); err != nil {
				logrus.Warnf("Failed to encode json: %v", err)
			}
			flush()
		case <-pushCtx.Done():
			if pushError != nil {
				report.Error = pushError.Error()
				if err := enc.Encode(report); err != nil {
					logrus.Warnf("Failed to encode json: %v", err)
				}
			}
			flush()
			return
		case <-r.Context().Done():
			// Client has closed connection
			return
		}
	}
}
