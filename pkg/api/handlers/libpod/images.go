package libpod

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/containers/buildah"
	"github.com/containers/common/libimage"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/api/handlers"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/auth"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/domain/infra/abi"
	"github.com/containers/podman/v4/pkg/errorhandling"
	"github.com/containers/podman/v4/pkg/util"
	utils2 "github.com/containers/podman/v4/utils"
	"github.com/containers/storage"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

// Commit
// author string
// "container"
// repo string
// tag string
// message
// pause bool
// changes []string

// create

func ImageExists(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)

	ir := abi.ImageEngine{Libpod: runtime}
	report, err := ir.Exists(r.Context(), name)
	if err != nil {
		utils.Error(w, http.StatusNotFound, errors.Wrapf(err, "failed to find image %s", name))
		return
	}
	if !report.Value {
		utils.Error(w, http.StatusNotFound, errors.Errorf("failed to find image %s", name))
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}

func ImageTree(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		WhatRequires bool `schema:"whatrequires"`
	}{
		WhatRequires: false,
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	ir := abi.ImageEngine{Libpod: runtime}
	options := entities.ImageTreeOptions{WhatRequires: query.WhatRequires}
	report, err := ir.Tree(r.Context(), name, options)
	if err != nil {
		if errors.Cause(err) == storage.ErrImageUnknown {
			utils.Error(w, http.StatusNotFound, errors.Wrapf(err, "failed to find image %s", name))
			return
		}
		utils.Error(w, http.StatusInternalServerError, errors.Wrapf(err, "failed to generate image tree for %s", name))
		return
	}
	utils.WriteResponse(w, http.StatusOK, report)
}

func GetImage(w http.ResponseWriter, r *http.Request) {
	name := utils.GetName(r)
	newImage, err := utils.GetImage(r, name)
	if err != nil {
		utils.Error(w, http.StatusNotFound, errors.Wrapf(err, "failed to find image %s", name))
		return
	}
	options := &libimage.InspectOptions{WithParent: true, WithSize: true}
	inspect, err := newImage.Inspect(r.Context(), options)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrapf(err, "failed in inspect image %s", inspect.ID))
		return
	}
	utils.WriteResponse(w, http.StatusOK, inspect)
}

func PruneImages(w http.ResponseWriter, r *http.Request) {
	var err error
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		All      bool `schema:"all"`
		External bool `schema:"external"`
	}{
		// override any golang type defaults
	}

	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError,
			errors.
				Wrapf(err, "failed to decode filter parameters for %s", r.URL.String()))
		return
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusInternalServerError,
			errors.
				Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	libpodFilters := []string{}
	if _, found := r.URL.Query()["filters"]; found {
		dangling := (*filterMap)["all"]
		if len(dangling) > 0 {
			query.All, err = strconv.ParseBool((*filterMap)["all"][0])
			if err != nil {
				utils.InternalServerError(w, err)
				return
			}
		}
		// dangling is special and not implemented in the libpod side of things
		delete(*filterMap, "dangling")
		for k, v := range *filterMap {
			libpodFilters = append(libpodFilters, fmt.Sprintf("%s=%s", k, v[0]))
		}
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}

	pruneOptions := entities.ImagePruneOptions{
		All:      query.All,
		External: query.External,
		Filter:   libpodFilters,
	}
	imagePruneReports, err := imageEngine.Prune(r.Context(), pruneOptions)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, imagePruneReports)
}

func ExportImage(w http.ResponseWriter, r *http.Request) {
	var output string
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Compress bool   `schema:"compress"`
		Format   string `schema:"format"`
	}{
		Format: define.OCIArchive,
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	name := utils.GetName(r)

	if _, _, err := runtime.LibimageRuntime().LookupImage(name, nil); err != nil {
		utils.ImageNotFound(w, name, err)
		return
	}

	switch query.Format {
	case define.OCIArchive, define.V2s2Archive:
		tmpfile, err := ioutil.TempFile("", "api.tar")
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "unable to create tempfile"))
			return
		}
		output = tmpfile.Name()
		if err := tmpfile.Close(); err != nil {
			utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "unable to close tempfile"))
			return
		}
	case define.OCIManifestDir, define.V2s2ManifestDir:
		tmpdir, err := ioutil.TempDir("", "save")
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "unable to create tempdir"))
			return
		}
		output = tmpdir
	default:
		utils.Error(w, http.StatusInternalServerError, errors.Errorf("unknown format %q", query.Format))
		return
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}

	saveOptions := entities.ImageSaveOptions{
		Compress: query.Compress,
		Format:   query.Format,
		Output:   output,
	}
	if err := imageEngine.Save(r.Context(), name, nil, saveOptions); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}
	defer os.RemoveAll(output)
	// if dir format, we need to tar it
	if query.Format == "oci-dir" || query.Format == "docker-dir" {
		rdr, err := utils2.Tar(output)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		defer rdr.Close()
		utils.WriteResponse(w, http.StatusOK, rdr)
		return
	}
	rdr, err := os.Open(output)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "failed to read the exported tarfile"))
		return
	}
	defer rdr.Close()
	utils.WriteResponse(w, http.StatusOK, rdr)
}

func ExportImages(w http.ResponseWriter, r *http.Request) {
	var output string
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Compress                    bool     `schema:"compress"`
		Format                      string   `schema:"format"`
		OciAcceptUncompressedLayers bool     `schema:"ociAcceptUncompressedLayers"`
		References                  []string `schema:"references"`
	}{
		Format: define.OCIArchive,
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	// References are mandatory!
	if len(query.References) == 0 {
		utils.Error(w, http.StatusBadRequest, errors.New("No references"))
		return
	}

	// Format is mandatory! Currently, we only support multi-image docker
	// archives.
	if len(query.References) > 1 && query.Format != define.V2s2Archive {
		utils.Error(w, http.StatusInternalServerError, errors.Errorf("multi-image archives must use format of %s", define.V2s2Archive))
		return
	}

	// if format is dir, server will save to an archive
	// the client will unArchive after receive the archive file
	// so must convert is at here
	switch query.Format {
	case define.OCIManifestDir:
		query.Format = define.OCIArchive
	case define.V2s2ManifestDir:
		query.Format = define.V2s2Archive
	}

	switch query.Format {
	case define.V2s2Archive, define.OCIArchive:
		tmpfile, err := ioutil.TempFile("", "api.tar")
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "unable to create tempfile"))
			return
		}
		output = tmpfile.Name()
		if err := tmpfile.Close(); err != nil {
			utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "unable to close tempfile"))
			return
		}
	case define.OCIManifestDir, define.V2s2ManifestDir:
		tmpdir, err := ioutil.TempDir("", "save")
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "unable to create tmpdir"))
			return
		}
		output = tmpdir
	default:
		utils.Error(w, http.StatusInternalServerError, errors.Errorf("unsupported format %q", query.Format))
		return
	}
	defer os.RemoveAll(output)

	// Use the ABI image engine to share as much code as possible.
	opts := entities.ImageSaveOptions{
		Compress:                    query.Compress,
		Format:                      query.Format,
		MultiImageArchive:           len(query.References) > 1,
		OciAcceptUncompressedLayers: query.OciAcceptUncompressedLayers,
		Output:                      output,
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}
	if err := imageEngine.Save(r.Context(), query.References[0], query.References[1:], opts); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	rdr, err := os.Open(output)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "failed to read the exported tarfile"))
		return
	}
	defer rdr.Close()
	utils.WriteResponse(w, http.StatusOK, rdr)
}

func ImagesLoad(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	tmpfile, err := ioutil.TempFile("", "libpod-images-load.tar")
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "unable to create tempfile"))
		return
	}
	defer os.Remove(tmpfile.Name())

	_, err = io.Copy(tmpfile, r.Body)
	tmpfile.Close()

	if err != nil && err != io.EOF {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "unable to write archive to temporary file"))
		return
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}

	loadOptions := entities.ImageLoadOptions{Input: tmpfile.Name()}
	loadReport, err := imageEngine.Load(r.Context(), loadOptions)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "unable to load image"))
		return
	}
	utils.WriteResponse(w, http.StatusOK, loadReport)
}

func ImagesImport(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Changes      []string `schema:"changes"`
		Message      string   `schema:"message"`
		Reference    string   `schema:"reference"`
		URL          string   `schema:"URL"`
		OS           string   `schema:"OS"`
		Architecture string   `schema:"Architecture"`
		Variant      string   `schema:"Variant"`
	}{
		// Add defaults here once needed.
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	// Check if we need to load the image from a URL or from the request's body.
	source := query.URL
	if len(query.URL) == 0 {
		tmpfile, err := ioutil.TempFile("", "libpod-images-import.tar")
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "unable to create tempfile"))
			return
		}
		defer os.Remove(tmpfile.Name())
		defer tmpfile.Close()

		if _, err := io.Copy(tmpfile, r.Body); err != nil && err != io.EOF {
			utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "unable to write archive to temporary file"))
			return
		}

		tmpfile.Close()
		source = tmpfile.Name()
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}
	importOptions := entities.ImageImportOptions{
		Changes:      query.Changes,
		Message:      query.Message,
		Reference:    query.Reference,
		OS:           query.OS,
		Architecture: query.Architecture,
		Variant:      query.Variant,
		Source:       source,
	}
	report, err := imageEngine.Import(r.Context(), importOptions)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "unable to import tarball"))
		return
	}

	utils.WriteResponse(w, http.StatusOK, report)
}

// PushImage is the handler for the compat http endpoint for pushing images.
func PushImage(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		Destination string `schema:"destination"`
		TLSVerify   bool   `schema:"tlsVerify"`
		Format      string `schema:"format"`
		All         bool   `schema:"all"`
	}{
		// This is where you can override the golang default value for one of fields
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
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
		Authfile: authfile,
		Username: username,
		Password: password,
		Format:   query.Format,
		All:      query.All,
		Quiet:    true,
	}
	if _, found := r.URL.Query()["tlsVerify"]; found {
		options.SkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}
	if err := imageEngine.Push(context.Background(), source, destination, options); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "error pushing image %q", destination))
		return
	}

	utils.WriteResponse(w, http.StatusOK, "")
}

func CommitContainer(w http.ResponseWriter, r *http.Request) {
	var (
		destImage string
		mimeType  string
	)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		Author    string   `schema:"author"`
		Changes   []string `schema:"changes"`
		Comment   string   `schema:"comment"`
		Container string   `schema:"container"`
		Format    string   `schema:"format"`
		Pause     bool     `schema:"pause"`
		Squash    bool     `schema:"squash"`
		Repo      string   `schema:"repo"`
		Tag       string   `schema:"tag"`
	}{
		Format: "oci",
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	rtc, err := runtime.GetConfig()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "failed to get runtime config"))
		return
	}
	sc := runtime.SystemContext()
	tag := "latest"
	options := libpod.ContainerCommitOptions{
		Pause: true,
	}
	switch query.Format {
	case "oci":
		mimeType = buildah.OCIv1ImageManifest
		if len(query.Comment) > 0 {
			utils.InternalServerError(w, errors.New("messages are only compatible with the docker image format (-f docker)"))
			return
		}
	case "docker":
		mimeType = manifest.DockerV2Schema2MediaType
	default:
		utils.InternalServerError(w, errors.Errorf("unrecognized image format %q", query.Format))
		return
	}
	options.CommitOptions = buildah.CommitOptions{
		SignaturePolicyPath:   rtc.Engine.SignaturePolicyPath,
		ReportWriter:          os.Stderr,
		SystemContext:         sc,
		PreferredManifestType: mimeType,
	}

	if len(query.Tag) > 0 {
		tag = query.Tag
	}
	options.Message = query.Comment
	options.Author = query.Author
	options.Pause = query.Pause
	options.Squash = query.Squash
	options.Changes = query.Changes
	ctr, err := runtime.LookupContainer(query.Container)
	if err != nil {
		utils.Error(w, http.StatusNotFound, err)
		return
	}

	if len(query.Repo) > 0 {
		destImage = fmt.Sprintf("%s:%s", query.Repo, tag)
	}
	commitImage, err := ctr.Commit(r.Context(), destImage, options)
	if err != nil && !strings.Contains(err.Error(), "is not running") {
		utils.Error(w, http.StatusInternalServerError, errors.Wrapf(err, "CommitFailure"))
		return
	}
	utils.WriteResponse(w, http.StatusOK, entities.IDResponse{ID: commitImage.ID()}) // nolint
}

func UntagImage(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	tags := []string{} // Note: if empty, all tags will be removed from the image.
	repo := r.Form.Get("repo")
	tag := r.Form.Get("tag")

	// Do the parameter dance.
	switch {
	// If tag is set, repo must be as well.
	case len(repo) == 0 && len(tag) > 0:
		utils.Error(w, http.StatusBadRequest, errors.New("repo parameter is required to tag an image"))
		return

	case len(repo) == 0:
		break

	// If repo is specified, we need to add that to the tags.
	default:
		if len(tag) == 0 {
			// Normalize tag to "latest" if empty.
			tag = "latest"
		}
		tags = append(tags, fmt.Sprintf("%s:%s", repo, tag))
	}

	// Now use the ABI implementation to prevent us from having duplicate
	// code.
	opts := entities.ImageUntagOptions{}
	imageEngine := abi.ImageEngine{Libpod: runtime}

	name := utils.GetName(r)
	if err := imageEngine.Untag(r.Context(), name, tags, opts); err != nil {
		if errors.Cause(err) == storage.ErrImageUnknown {
			utils.ImageNotFound(w, name, errors.Wrapf(err, "failed to find image %s", name))
		} else {
			utils.Error(w, http.StatusInternalServerError, err)
		}
		return
	}
	utils.WriteResponse(w, http.StatusCreated, "")
}

// ImagesBatchRemove is the endpoint for batch image removal.
func ImagesBatchRemove(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		All    bool     `schema:"all"`
		Force  bool     `schema:"force"`
		Ignore bool     `schema:"ignore"`
		Images []string `schema:"images"`
	}{}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	opts := entities.ImageRemoveOptions{All: query.All, Force: query.Force, Ignore: query.Ignore}
	imageEngine := abi.ImageEngine{Libpod: runtime}
	rmReport, rmErrors := imageEngine.Remove(r.Context(), query.Images, opts)
	strErrs := errorhandling.ErrorsToStrings(rmErrors)
	report := handlers.LibpodImagesRemoveReport{ImageRemoveReport: *rmReport, Errors: strErrs}
	utils.WriteResponse(w, http.StatusOK, report)
}

// ImagesRemove is the endpoint for removing one image.
func ImagesRemove(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Force bool `schema:"force"`
	}{
		Force: false,
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	opts := entities.ImageRemoveOptions{Force: query.Force}
	imageEngine := abi.ImageEngine{Libpod: runtime}
	rmReport, rmErrors := imageEngine.Remove(r.Context(), []string{utils.GetName(r)}, opts)

	// In contrast to batch-removal, where we're only setting the exit
	// code, we need to have another closer look at the errors here and set
	// the appropriate http status code.

	switch rmReport.ExitCode {
	case 0:
		report := handlers.LibpodImagesRemoveReport{ImageRemoveReport: *rmReport, Errors: []string{}}
		utils.WriteResponse(w, http.StatusOK, report)
	case 1:
		// 404 - no such image
		utils.Error(w, http.StatusNotFound, errorhandling.JoinErrors(rmErrors))
	case 2:
		// 409 - conflict error (in use by containers)
		utils.Error(w, http.StatusConflict, errorhandling.JoinErrors(rmErrors))
	default:
		// 500 - internal error
		utils.Error(w, http.StatusInternalServerError, errorhandling.JoinErrors(rmErrors))
	}
}
