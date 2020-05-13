package libpod

import (
	"encoding/json"
	"net/http"

	"github.com/containers/buildah/manifests"
	copy2 "github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/api/handlers/utils"
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
	newImage, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		utils.ImageNotFound(w, name, err)
		return
	}
	data, err := newImage.InspectManifest()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, data)
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
	// FIXME: parameters are missing (tlsVerify, format).
	// Also, we should use the ABI function to avoid duplicate code.
	// Also, support for XRegistryAuth headers are missing.

	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		All         bool   `schema:"all"`
		Destination string `schema:"destination"`
	}{
		// Add defaults here once needed.
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	name := utils.GetName(r)
	newImage, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		utils.ImageNotFound(w, name, err)
		return
	}
	dest, err := alltransports.ParseImageName(query.Destination)
	if err != nil {
		utils.Error(w, "invalid destination parameter", http.StatusBadRequest, errors.Errorf("invalid destination parameter %q", query.Destination))
		return
	}
	rtc, err := runtime.GetConfig()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	sc := image.GetSystemContext(rtc.Engine.SignaturePolicyPath, "", false)
	opts := manifests.PushOptions{
		Store:              runtime.GetStore(),
		ImageListSelection: copy2.CopySpecificImages,
		SystemContext:      sc,
	}
	if query.All {
		opts.ImageListSelection = copy2.CopyAllImages
	}
	newD, err := newImage.PushManifest(dest, opts)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, newD.String())
}
