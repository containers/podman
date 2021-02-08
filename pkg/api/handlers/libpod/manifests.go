package libpod

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/image"
	"github.com/containers/podman/v2/pkg/api/handlers"
	"github.com/containers/podman/v2/pkg/api/handlers/utils"
	"github.com/containers/podman/v2/pkg/auth"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/domain/infra/abi"
	"github.com/gorilla/schema"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

func ManifestCreate(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Name  []string `schema:"name"`
		Image []string `schema:"image"`
		All   bool     `schema:"all"`
	}{
		// Add defaults here once needed.
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	rtc, err := runtime.GetConfig()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	sc := image.GetSystemContext(rtc.Engine.SignaturePolicyPath, "", false)
	manID, err := image.CreateManifestList(runtime.ImageRuntime(), *sc, query.Name, query.Image, query.All)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, handlers.IDResponse{ID: manID})
}

func ManifestInspect(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := utils.GetName(r)
	imageEngine := abi.ImageEngine{Libpod: runtime}
	inspectReport, inspectError := imageEngine.ManifestInspect(r.Context(), name)
	if inspectError != nil {
		utils.Error(w, "Something went wrong.", http.StatusNotFound, inspectError)
		return
	}
	var list manifest.Schema2List
	if err := json.Unmarshal(inspectReport, &list); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Unmarshal()"))
		return
	}
	utils.WriteResponse(w, http.StatusOK, &list)
}

func ManifestAdd(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	var manifestInput image.ManifestAddOpts
	if err := json.NewDecoder(r.Body).Decode(&manifestInput); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}
	name := utils.GetName(r)
	newImage, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		utils.ImageNotFound(w, name, err)
		return
	}
	rtc, err := runtime.GetConfig()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	sc := image.GetSystemContext(rtc.Engine.SignaturePolicyPath, "", false)
	newID, err := newImage.AddManifest(*sc, manifestInput)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, handlers.IDResponse{ID: newID})
}

func ManifestRemove(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Digest string `schema:"digest"`
	}{
		// Add defaults here once needed.
	}
	name := utils.GetName(r)
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	newImage, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		utils.ImageNotFound(w, name, err)
		return
	}
	d, err := digest.Parse(query.Digest)
	if err != nil {
		utils.Error(w, "invalid digest", http.StatusBadRequest, err)
		return
	}
	newID, err := newImage.RemoveManifest(d)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, handlers.IDResponse{ID: newID})
}
func ManifestPush(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		All         bool   `schema:"all"`
		Destination string `schema:"destination"`
		TLSVerify   bool   `schema:"tlsVerify"`
	}{
		// Add defaults here once needed.
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	if _, err := utils.ParseDockerReference(query.Destination); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest, err)
		return
	}

	source := utils.GetName(r)
	authconf, authfile, key, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, "failed to retrieve repository credentials", http.StatusBadRequest, errors.Wrapf(err, "failed to parse %q header for %s", key, r.URL.String()))
		return
	}
	defer auth.RemoveAuthfile(authfile)
	var username, password string
	if authconf != nil {
		username = authconf.Username
		password = authconf.Password
	}
	options := entities.ImagePushOptions{
		Authfile: authfile,
		Username: username,
		Password: password,
		All:      query.All,
	}
	if sys := runtime.SystemContext(); sys != nil {
		options.CertDir = sys.DockerCertPath
	}
	if _, found := r.URL.Query()["tlsVerify"]; found {
		options.SkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}
	imageEngine := abi.ImageEngine{Libpod: runtime}
	digest, err := imageEngine.ManifestPush(context.Background(), source, query.Destination, options)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "error pushing image %q", query.Destination))
		return
	}
	utils.WriteResponse(w, http.StatusOK, digest)
}
