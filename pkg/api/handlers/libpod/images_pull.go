package libpod

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/containers/common/libimage"
	"github.com/containers/common/pkg/config"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/auth"
	"github.com/containers/podman/v4/pkg/channel"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/gorilla/schema"
	"github.com/sirupsen/logrus"
)

// ImagesPull is the v2 libpod endpoint for pulling images.  Note that the
// mandatory `reference` must be a reference to a registry (i.e., of docker
// transport or be normalized to one).  Other transports are rejected as they
// do not make sense in a remote context.
func ImagesPull(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Reference  string `schema:"reference"`
		OS         string `schema:"OS"`
		Arch       string `schema:"Arch"`
		Variant    string `schema:"Variant"`
		TLSVerify  bool   `schema:"tlsVerify"`
		AllTags    bool   `schema:"allTags"`
		PullPolicy string `schema:"policy"`
		Quiet      bool   `schema:"quiet"`
	}{
		TLSVerify:  true,
		PullPolicy: "always",
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	if len(query.Reference) == 0 {
		utils.InternalServerError(w, errors.New("reference parameter cannot be empty"))
		return
	}

	// Make sure that the reference has no transport or the docker one.
	if err := utils.IsRegistryReference(query.Reference); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	pullOptions := &libimage.PullOptions{}
	pullOptions.AllTags = query.AllTags
	pullOptions.Architecture = query.Arch
	pullOptions.OS = query.OS
	pullOptions.Variant = query.Variant

	if _, found := r.URL.Query()["tlsVerify"]; found {
		pullOptions.InsecureSkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}

	// Do the auth dance.
	authConf, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}
	defer auth.RemoveAuthfile(authfile)

	pullOptions.AuthFilePath = authfile
	if authConf != nil {
		pullOptions.Username = authConf.Username
		pullOptions.Password = authConf.Password
		pullOptions.IdentityToken = authConf.IdentityToken
	}

	pullPolicy, err := config.ParsePullPolicy(query.PullPolicy)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	// Let's keep thing simple when running in quiet mode and pull directly.
	if query.Quiet {
		images, err := runtime.LibimageRuntime().Pull(r.Context(), query.Reference, pullPolicy, pullOptions)
		var report entities.ImagePullReport
		if err != nil {
			report.Error = err.Error()
		}
		for _, image := range images {
			report.Images = append(report.Images, image.ID())
			// Pull last ID from list and publish in 'id' stanza.  This maintains previous API contract
			report.ID = image.ID()
		}
		utils.WriteResponse(w, http.StatusOK, report)
		return
	}

	writer := channel.NewWriter(make(chan []byte))
	defer writer.Close()
	pullOptions.Writer = writer

	var pulledImages []*libimage.Image
	var pullError error
	runCtx, cancel := context.WithCancel(r.Context())
	go func() {
		defer cancel()
		pulledImages, pullError = runtime.LibimageRuntime().Pull(runCtx, query.Reference, pullPolicy, pullOptions)
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
		var report entities.ImagePullReport
		select {
		case s := <-writer.Chan():
			report.Stream = string(s)
			if err := enc.Encode(report); err != nil {
				logrus.Warnf("Failed to encode json: %v", err)
			}
			flush()
		case <-runCtx.Done():
			for _, image := range pulledImages {
				report.Images = append(report.Images, image.ID())
				// Pull last ID from list and publish in 'id' stanza.  This maintains previous API contract
				report.ID = image.ID()
			}
			if pullError != nil {
				report.Error = pullError.Error()
			}
			if err := enc.Encode(report); err != nil {
				logrus.Warnf("Failed to encode json: %v", err)
			}
			flush()
			return
		case <-r.Context().Done():
			// Client has closed connection
			return
		}
	}
}
