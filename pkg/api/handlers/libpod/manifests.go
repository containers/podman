package libpod

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/api/handlers"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/auth"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/infra/abi"
	"github.com/containers/podman/v4/pkg/errorhandling"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

func ManifestCreate(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Name   string   `schema:"name"`
		Images []string `schema:"images"`
		All    bool     `schema:"all"`
	}{
		// Add defaults here once needed.
	}

	// Support 3.x API calls, alias image to images
	if image, ok := r.URL.Query()["image"]; ok {
		query.Images = image
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	// Support 4.x API calls, map query parameter to path
	if name, ok := mux.Vars(r)["name"]; ok {
		n, err := url.QueryUnescape(name)
		if err != nil {
			utils.Error(w, http.StatusBadRequest,
				errors.Wrapf(err, "failed to parse name parameter %q", name))
			return
		}
		query.Name = n
	}

	if _, err := reference.ParseNormalizedNamed(query.Name); err != nil {
		utils.Error(w, http.StatusBadRequest,
			errors.Wrapf(err, "invalid image name %s", query.Name))
		return
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}

	createOptions := entities.ManifestCreateOptions{All: query.All}
	manID, err := imageEngine.ManifestCreate(r.Context(), query.Name, query.Images, createOptions)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	status := http.StatusOK
	if _, err := utils.SupportedVersion(r, "< 4.0.0"); err == utils.ErrVersionNotSupported {
		status = http.StatusCreated
	}

	buffer, err := ioutil.ReadAll(r.Body)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	// Treat \r\n as empty body
	if len(buffer) < 3 {
		utils.WriteResponse(w, status, entities.IDResponse{ID: manID})
		return
	}

	body := new(entities.ManifestModifyOptions)
	if err := json.Unmarshal(buffer, body); err != nil {
		utils.InternalServerError(w, errors.Wrap(err, "Decode()"))
		return
	}

	// gather all images for manifest list
	var images []string
	if len(query.Images) > 0 {
		images = query.Images
	}
	if len(body.Images) > 0 {
		images = body.Images
	}

	id, err := imageEngine.ManifestAdd(r.Context(), query.Name, images, body.ManifestAddOptions)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	utils.WriteResponse(w, status, entities.IDResponse{ID: id})
}

// ManifestExists return true if manifest list exists.
func ManifestExists(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)

	imageEngine := abi.ImageEngine{Libpod: runtime}
	report, err := imageEngine.ManifestExists(r.Context(), name)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	if !report.Value {
		utils.Error(w, http.StatusNotFound, errors.New("manifest not found"))
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}

func ManifestInspect(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)

	imageEngine := abi.ImageEngine{Libpod: runtime}
	rawManifest, err := imageEngine.ManifestInspect(r.Context(), name)
	if err != nil {
		utils.Error(w, http.StatusNotFound, err)
		return
	}

	var schema2List manifest.Schema2List
	if err := json.Unmarshal(rawManifest, &schema2List); err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}

	utils.WriteResponse(w, http.StatusOK, schema2List)
}

// ManifestAddV3 remove digest from manifest list
//
// As of 4.0.0 use ManifestModify instead
func ManifestAddV3(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	// Wrapper to support 3.x with 4.x libpod
	query := struct {
		entities.ManifestAddOptions
		TLSVerify bool `schema:"tlsVerify"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
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
	query.ManifestAddOptions.Authfile = authfile
	query.ManifestAddOptions.Username = username
	query.ManifestAddOptions.Password = password
	if sys := runtime.SystemContext(); sys != nil {
		query.ManifestAddOptions.CertDir = sys.DockerCertPath
	}
	if _, found := r.URL.Query()["tlsVerify"]; found {
		query.SkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}

	name := utils.GetName(r)
	if _, err := runtime.LibimageRuntime().LookupManifestList(name); err != nil {
		utils.Error(w, http.StatusNotFound, err)
		return
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}
	newID, err := imageEngine.ManifestAdd(r.Context(), name, query.Images, query.ManifestAddOptions)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, entities.IDResponse{ID: newID})
}

// ManifestRemoveDigestV3 remove digest from manifest list
//
// As of 4.0.0 use ManifestModify instead
func ManifestRemoveDigestV3(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Digest string `schema:"digest"`
	}{
		// Add defaults here once needed.
	}
	name := utils.GetName(r)
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	manifestList, err := runtime.LibimageRuntime().LookupManifestList(name)
	if err != nil {
		utils.Error(w, http.StatusNotFound, err)
		return
	}
	d, err := digest.Parse(query.Digest)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}
	if err := manifestList.RemoveInstance(d); err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, entities.IDResponse{ID: manifestList.ID()})
}

// ManifestPushV3 push image to registry
//
// As of 4.0.0 use ManifestPush instead
func ManifestPushV3(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		All         bool   `schema:"all"`
		Destination string `schema:"destination"`
		TLSVerify   bool   `schema:"tlsVerify"`
	}{
		// Add defaults here once needed.
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	if err := utils.IsRegistryReference(query.Destination); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	source := utils.GetName(r)
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
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "error pushing image %q", query.Destination))
		return
	}
	utils.WriteResponse(w, http.StatusOK, entities.IDResponse{ID: digest})
}

// ManifestPush push image to registry
//
// As of 4.0.0
func ManifestPush(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)

	query := struct {
		All       bool `schema:"all"`
		TLSVerify bool `schema:"tlsVerify"`
	}{
		// Add defaults here once needed.
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	destination := utils.GetVar(r, "destination")
	if err := utils.IsRegistryReference(destination); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	authconf, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse registry header for %s", r.URL.String()))
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
	source := utils.GetName(r)
	digest, err := imageEngine.ManifestPush(context.Background(), source, destination, options)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "error pushing image %q", destination))
		return
	}
	utils.WriteResponse(w, http.StatusOK, entities.IDResponse{ID: digest})
}

// ManifestModify efficiently updates the named manifest list
func ManifestModify(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	imageEngine := abi.ImageEngine{Libpod: runtime}

	body := new(entities.ManifestModifyOptions)
	if err := json.NewDecoder(r.Body).Decode(body); err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}

	name := utils.GetName(r)
	if _, err := runtime.LibimageRuntime().LookupManifestList(name); err != nil {
		utils.Error(w, http.StatusNotFound, err)
		return
	}

	if tlsVerify, ok := r.URL.Query()["tlsVerify"]; ok {
		tls, err := strconv.ParseBool(tlsVerify[len(tlsVerify)-1])
		if err != nil {
			utils.Error(w, http.StatusBadRequest, fmt.Errorf("tlsVerify param is not a bool: %w", err))
			return
		}
		body.SkipTLSVerify = types.NewOptionalBool(!tls)
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
	body.ManifestAddOptions.Authfile = authfile
	body.ManifestAddOptions.Username = username
	body.ManifestAddOptions.Password = password
	if sys := runtime.SystemContext(); sys != nil {
		body.ManifestAddOptions.CertDir = sys.DockerCertPath
	}

	var report entities.ManifestModifyReport
	switch {
	case strings.EqualFold("update", body.Operation):
		id, err := imageEngine.ManifestAdd(r.Context(), name, body.Images, body.ManifestAddOptions)
		if err != nil {
			report.Errors = append(report.Errors, err)
			break
		}
		report = entities.ManifestModifyReport{
			ID:     id,
			Images: body.Images,
		}
	case strings.EqualFold("remove", body.Operation):
		for _, image := range body.Images {
			id, err := imageEngine.ManifestRemoveDigest(r.Context(), name, image)
			if err != nil {
				report.Errors = append(report.Errors, err)
				continue
			}
			report.ID = id
			report.Images = append(report.Images, image)
		}
	case strings.EqualFold("annotate", body.Operation):
		options := entities.ManifestAnnotateOptions{
			Annotation: body.Annotation,
			Arch:       body.Arch,
			Features:   body.Features,
			OS:         body.OS,
			OSFeatures: body.OSFeatures,
			OSVersion:  body.OSVersion,
			Variant:    body.Variant,
		}
		for _, image := range body.Images {
			id, err := imageEngine.ManifestAnnotate(r.Context(), name, image, options)
			if err != nil {
				report.Errors = append(report.Errors, err)
				continue
			}
			report.ID = id
			report.Images = append(report.Images, image)
		}
	default:
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("illegal operation %q for %q", body.Operation, r.URL.String()))
		return
	}

	statusCode := http.StatusOK
	switch {
	case len(report.Errors) > 0 && len(report.Images) > 0:
		statusCode = http.StatusConflict
	case len(report.Errors) > 0:
		statusCode = http.StatusBadRequest
	}
	utils.WriteResponse(w, statusCode, report)
}

// ManifestDelete removes a manifest list from storage
func ManifestDelete(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	imageEngine := abi.ImageEngine{Libpod: runtime}

	name := utils.GetName(r)
	if _, err := runtime.LibimageRuntime().LookupManifestList(name); err != nil {
		utils.Error(w, http.StatusNotFound, err)
		return
	}

	results, errs := imageEngine.ManifestRm(r.Context(), []string{name})
	errsString := errorhandling.ErrorsToStrings(errs)
	report := handlers.LibpodImagesRemoveReport{
		ImageRemoveReport: *results,
		Errors:            errsString,
	}
	utils.WriteResponse(w, http.StatusOK, report)
}
