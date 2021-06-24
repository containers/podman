package libpod

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/pkg/api/handlers"
	"github.com/containers/podman/v3/pkg/api/handlers/utils"
	"github.com/containers/podman/v3/pkg/auth"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/domain/infra/abi"
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

	// TODO: (jhonce) When c/image is refactored the roadmap calls for this check to be pushed into that library.
	for _, n := range query.Name {
		if _, err := reference.ParseNormalizedNamed(n); err != nil {
			utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
				errors.Wrapf(err, "invalid image name %s", n))
			return
		}
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}

	createOptions := entities.ManifestCreateOptions{All: query.All}
	manID, err := imageEngine.ManifestCreate(r.Context(), query.Name, query.Image, createOptions)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, handlers.IDResponse{ID: manID})
}

// ExistsManifest check if a manifest list exists
func ExistsManifest(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := utils.GetName(r)

	imageEngine := abi.ImageEngine{Libpod: runtime}
	report, err := imageEngine.ManifestExists(r.Context(), name)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	if !report.Value {
		utils.Error(w, "manifest not found", http.StatusNotFound, errors.New("manifest not found"))
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}

func ManifestInspect(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := utils.GetName(r)

	imageEngine := abi.ImageEngine{Libpod: runtime}
	rawManifest, err := imageEngine.ManifestInspect(r.Context(), name)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusNotFound, err)
		return
	}

	var schema2List manifest.Schema2List
	if err := json.Unmarshal(rawManifest, &schema2List); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}

	utils.WriteResponse(w, http.StatusOK, schema2List)
}

func ManifestAdd(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	var addOptions entities.ManifestAddOptions
	if err := json.NewDecoder(r.Body).Decode(&addOptions); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}

	name := utils.GetName(r)
	if _, err := runtime.LibimageRuntime().LookupManifestList(name); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusNotFound, err)
		return
	}

	// FIXME: we really need to clean up the manifest API.  Swagger states
	// the arguments were strings not string slices.  The use of string
	// slices, mixing lists and images is incredibly confusing.
	if len(addOptions.Images) == 1 {
		addOptions.Images = append(addOptions.Images, name)
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}
	newID, err := imageEngine.ManifestAdd(r.Context(), addOptions)
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
	manifestList, err := runtime.LibimageRuntime().LookupManifestList(name)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusNotFound, err)
		return
	}
	d, err := digest.Parse(query.Digest)
	if err != nil {
		utils.Error(w, "invalid digest", http.StatusBadRequest, err)
		return
	}
	if err := manifestList.RemoveInstance(d); err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, handlers.IDResponse{ID: manifestList.ID()})
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
	if err := utils.IsRegistryReference(query.Destination); err != nil {
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
