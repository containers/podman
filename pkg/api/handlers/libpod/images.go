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
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/libpod/image"
	image2 "github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/containers/libpod/pkg/auth"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/domain/infra/abi"
	"github.com/containers/libpod/pkg/errorhandling"
	"github.com/containers/libpod/pkg/util"
	utils2 "github.com/containers/libpod/utils"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := utils.GetName(r)

	_, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusNotFound, errors.Wrapf(err, "Failed to find image %s", name))
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}

func ImageTree(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := utils.GetName(r)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		WhatRequires bool `schema:"whatrequires"`
	}{
		WhatRequires: false,
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	ir := abi.ImageEngine{Libpod: runtime}
	options := entities.ImageTreeOptions{WhatRequires: query.WhatRequires}
	report, err := ir.Tree(r.Context(), name, options)
	if err != nil {
		if errors.Cause(err) == define.ErrNoSuchImage {
			utils.Error(w, "Something went wrong.", http.StatusNotFound, errors.Wrapf(err, "Failed to find image %s", name))
			return
		}
		utils.Error(w, "Server error", http.StatusInternalServerError, errors.Wrapf(err, "failed to generate image tree for %s", name))
		return
	}
	utils.WriteResponse(w, http.StatusOK, report)
}

func GetImage(w http.ResponseWriter, r *http.Request) {
	name := utils.GetName(r)
	newImage, err := utils.GetImage(r, name)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusNotFound, errors.Wrapf(err, "Failed to find image %s", name))
		return
	}
	inspect, err := newImage.Inspect(r.Context())
	if err != nil {
		utils.Error(w, "Server error", http.StatusInternalServerError, errors.Wrapf(err, "failed in inspect image %s", inspect.ID))
		return
	}
	utils.WriteResponse(w, http.StatusOK, inspect)
}

func GetImages(w http.ResponseWriter, r *http.Request) {
	images, err := utils.GetImages(w, r)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Failed get images"))
		return
	}
	var summaries = make([]*entities.ImageSummary, len(images))
	for j, img := range images {
		is, err := handlers.ImageToImageSummary(img)
		if err != nil {
			utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Failed transform image summaries"))
			return
		}
		// libpod has additional fields that we need to populate.
		is.Created = img.Created()
		is.ReadOnly = img.IsReadOnly()
		summaries[j] = is
	}
	utils.WriteResponse(w, http.StatusOK, summaries)
}

func PruneImages(w http.ResponseWriter, r *http.Request) {
	var (
		err error
	)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		All     bool                `schema:"all"`
		Filters map[string][]string `schema:"filters"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	var libpodFilters = []string{}
	if _, found := r.URL.Query()["filters"]; found {
		dangling := query.Filters["all"]
		if len(dangling) > 0 {
			query.All, err = strconv.ParseBool(query.Filters["all"][0])
			if err != nil {
				utils.InternalServerError(w, err)
				return
			}
		}
		// dangling is special and not implemented in the libpod side of things
		delete(query.Filters, "dangling")
		for k, v := range query.Filters {
			libpodFilters = append(libpodFilters, fmt.Sprintf("%s=%s", k, v[0]))
		}
	}

	cids, err := runtime.ImageRuntime().PruneImages(r.Context(), query.All, libpodFilters)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, cids)
}

func ExportImage(w http.ResponseWriter, r *http.Request) {
	var (
		output string
	)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Compress bool   `schema:"compress"`
		Format   string `schema:"format"`
	}{
		Format: define.OCIArchive,
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	switch query.Format {
	case define.OCIArchive, define.V2s2Archive:
		tmpfile, err := ioutil.TempFile("", "api.tar")
		if err != nil {
			utils.Error(w, "unable to create tmpfile", http.StatusInternalServerError, errors.Wrap(err, "unable to create tempfile"))
			return
		}
		output = tmpfile.Name()
		if err := tmpfile.Close(); err != nil {
			utils.Error(w, "unable to close tmpfile", http.StatusInternalServerError, errors.Wrap(err, "unable to close tempfile"))
			return
		}
	case define.OCIManifestDir, define.V2s2ManifestDir:
		tmpdir, err := ioutil.TempDir("", "save")
		if err != nil {
			utils.Error(w, "unable to create tmpdir", http.StatusInternalServerError, errors.Wrap(err, "unable to create tempdir"))
			return
		}
		output = tmpdir
	default:
		utils.Error(w, "unknown format", http.StatusInternalServerError, errors.Errorf("unknown format %q", query.Format))
		return
	}
	name := utils.GetName(r)
	newImage, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		utils.ImageNotFound(w, name, err)
		return
	}

	if err := newImage.Save(r.Context(), name, query.Format, output, []string{}, false, query.Compress); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest, err)
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
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to read the exported tarfile"))
		return
	}
	defer rdr.Close()
	utils.WriteResponse(w, http.StatusOK, rdr)
}

func ImagesLoad(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Reference string `schema:"reference"`
	}{
		// Add defaults here once needed.
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	tmpfile, err := ioutil.TempFile("", "libpod-images-load.tar")
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to create tempfile"))
		return
	}
	defer os.Remove(tmpfile.Name())
	defer tmpfile.Close()

	if _, err := io.Copy(tmpfile, r.Body); err != nil && err != io.EOF {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to write archive to temporary file"))
		return
	}

	tmpfile.Close()
	loadedImage, err := runtime.LoadImage(context.Background(), query.Reference, tmpfile.Name(), os.Stderr, "")
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to load image"))
		return
	}
	split := strings.Split(loadedImage, ",")
	newImage, err := runtime.ImageRuntime().NewFromLocal(split[0])
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	// TODO this should go into libpod proper at some point.
	if len(query.Reference) > 0 {
		if err := newImage.TagImage(query.Reference); err != nil {
			utils.InternalServerError(w, err)
			return
		}
	}
	utils.WriteResponse(w, http.StatusOK, entities.ImageLoadReport{Names: split})
}

func ImagesImport(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Changes   []string `schema:"changes"`
		Message   string   `schema:"message"`
		Reference string   `schema:"reference"`
		URL       string   `schema:"URL"`
	}{
		// Add defaults here once needed.
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	// Check if we need to load the image from a URL or from the request's body.
	source := query.URL
	if len(query.URL) == 0 {
		tmpfile, err := ioutil.TempFile("", "libpod-images-import.tar")
		if err != nil {
			utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to create tempfile"))
			return
		}
		defer os.Remove(tmpfile.Name())
		defer tmpfile.Close()

		if _, err := io.Copy(tmpfile, r.Body); err != nil && err != io.EOF {
			utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to write archive to temporary file"))
			return
		}

		tmpfile.Close()
		source = tmpfile.Name()
	}
	importedImage, err := runtime.Import(context.Background(), source, query.Reference, query.Changes, query.Message, true)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to import image"))
		return
	}

	utils.WriteResponse(w, http.StatusOK, entities.ImageImportReport{Id: importedImage})
}

// ImagesPull is the v2 libpod endpoint for pulling images.  Note that the
// mandatory `reference` must be a reference to a registry (i.e., of docker
// transport or be normalized to one).  Other transports are rejected as they
// do not make sense in a remote context.
func ImagesPull(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Reference    string `schema:"reference"`
		OverrideOS   string `schema:"overrideOS"`
		OverrideArch string `schema:"overrideArch"`
		TLSVerify    bool   `schema:"tlsVerify"`
		AllTags      bool   `schema:"allTags"`
	}{
		TLSVerify: true,
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	if len(query.Reference) == 0 {
		utils.InternalServerError(w, errors.New("reference parameter cannot be empty"))
		return
	}

	imageRef, err := utils.ParseDockerReference(query.Reference)
	if err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "image destination %q is not a docker-transport reference", query.Reference))
		return
	}

	// Trim the docker-transport prefix.
	rawImage := strings.TrimPrefix(query.Reference, fmt.Sprintf("%s://", docker.Transport.Name()))

	// all-tags doesn't work with a tagged reference, so let's check early
	namedRef, err := reference.Parse(rawImage)
	if err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "error parsing reference %q", rawImage))
		return
	}
	if _, isTagged := namedRef.(reference.Tagged); isTagged && query.AllTags {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Errorf("reference %q must not have a tag for all-tags", rawImage))
		return
	}

	authConf, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse %q header for %s", auth.XRegistryAuthHeader, r.URL.String()))
		return
	}
	defer auth.RemoveAuthfile(authfile)

	// Setup the registry options
	dockerRegistryOptions := image.DockerRegistryOptions{
		DockerRegistryCreds: authConf,
		OSChoice:            query.OverrideOS,
		ArchitectureChoice:  query.OverrideArch,
	}
	if _, found := r.URL.Query()["tlsVerify"]; found {
		dockerRegistryOptions.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}

	sys := runtime.SystemContext()
	if sys == nil {
		sys = image.GetSystemContext("", authfile, false)
	}
	dockerRegistryOptions.DockerCertPath = sys.DockerCertPath
	sys.DockerAuthConfig = authConf

	// Prepare the images we want to pull
	imagesToPull := []string{}
	res := []handlers.LibpodImagesPullReport{}
	imageName := namedRef.String()

	if !query.AllTags {
		imagesToPull = append(imagesToPull, imageName)
	} else {
		tags, err := docker.GetRepositoryTags(context.Background(), sys, imageRef)
		if err != nil {
			utils.InternalServerError(w, errors.Wrap(err, "error getting repository tags"))
			return
		}
		for _, tag := range tags {
			imagesToPull = append(imagesToPull, fmt.Sprintf("%s:%s", imageName, tag))
		}
	}

	// Finally pull the images
	for _, img := range imagesToPull {
		newImage, err := runtime.ImageRuntime().New(
			context.Background(),
			img,
			"",
			authfile,
			os.Stderr,
			&dockerRegistryOptions,
			image.SigningOptions{},
			nil,
			util.PullImageAlways)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		res = append(res, handlers.LibpodImagesPullReport{ID: newImage.ID()})
	}

	utils.WriteResponse(w, http.StatusOK, res)
}

// PushImage is the handler for the compat http endpoint for pushing images.
func PushImage(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	query := struct {
		Destination string `schema:"destination"`
		TLSVerify   bool   `schema:"tlsVerify"`
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	source := strings.TrimSuffix(utils.GetName(r), "/push") // GetName returns the entire path
	if _, err := utils.ParseStorageReference(source); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "image source %q is not a containers-storage-transport reference", source))
		return
	}

	destination := query.Destination
	if destination == "" {
		destination = source
	}

	if _, err := utils.ParseDockerReference(destination); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "image destination %q is not a docker-transport reference", destination))
		return
	}

	newImage, err := runtime.ImageRuntime().NewFromLocal(source)
	if err != nil {
		utils.ImageNotFound(w, source, errors.Wrapf(err, "Failed to find image %s", source))
		return
	}

	authConf, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse %q header for %s", auth.XRegistryAuthHeader, r.URL.String()))
		return
	}
	defer auth.RemoveAuthfile(authfile)
	logrus.Errorf("AuthConf: %v", authConf)

	dockerRegistryOptions := &image.DockerRegistryOptions{
		DockerRegistryCreds: authConf,
	}
	if sys := runtime.SystemContext(); sys != nil {
		dockerRegistryOptions.DockerCertPath = sys.DockerCertPath
		dockerRegistryOptions.RegistriesConfPath = sys.SystemRegistriesConfPath
	}
	if _, found := r.URL.Query()["tlsVerify"]; found {
		dockerRegistryOptions.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}

	err = newImage.PushImageToHeuristicDestination(
		context.Background(),
		destination,
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
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Error pushing image %q", destination))
		return
	}

	utils.WriteResponse(w, http.StatusOK, "")
}

func CommitContainer(w http.ResponseWriter, r *http.Request) {
	var (
		destImage string
		mimeType  string
	)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	query := struct {
		Author    string   `schema:"author"`
		Changes   []string `schema:"changes"`
		Comment   string   `schema:"comment"`
		Container string   `schema:"container"`
		Format    string   `schema:"format"`
		Pause     bool     `schema:"pause"`
		Repo      string   `schema:"repo"`
		Tag       string   `schema:"tag"`
	}{
		Format: "oci",
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	rtc, err := runtime.GetConfig()
	if err != nil {
		utils.Error(w, "failed to get runtime config", http.StatusInternalServerError, errors.Wrap(err, "failed to get runtime config"))
		return
	}
	sc := image2.GetSystemContext(rtc.Engine.SignaturePolicyPath, "", false)
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
	options.Changes = query.Changes
	ctr, err := runtime.LookupContainer(query.Container)
	if err != nil {
		utils.Error(w, "failed to lookup container", http.StatusNotFound, err)
		return
	}

	// I know mitr hates this ... but doing for now
	if len(query.Repo) > 1 {
		destImage = fmt.Sprintf("%s:%s", query.Repo, tag)
	}

	commitImage, err := ctr.Commit(r.Context(), destImage, options)
	if err != nil && !strings.Contains(err.Error(), "is not running") {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "CommitFailure"))
		return
	}
	utils.WriteResponse(w, http.StatusOK, handlers.IDResponse{ID: commitImage.ID()}) // nolint
}

func UntagImage(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	name := utils.GetName(r)
	newImage, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		utils.ImageNotFound(w, name, errors.Wrapf(err, "Failed to find image %s", name))
		return
	}
	tag := "latest"
	if len(r.Form.Get("tag")) > 0 {
		tag = r.Form.Get("tag")
	}
	if len(r.Form.Get("repo")) < 1 {
		utils.Error(w, "repo tag is required", http.StatusBadRequest, errors.New("repo parameter is required to tag an image"))
		return
	}
	repo := r.Form.Get("repo")
	tagName := fmt.Sprintf("%s:%s", repo, tag)
	if err := newImage.UntagImage(tagName); err != nil {
		utils.Error(w, "failed to untag", http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusCreated, "")
}

func SearchImages(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Term      string   `json:"term"`
		Limit     int      `json:"limit"`
		Filters   []string `json:"filters"`
		TLSVerify bool     `json:"tlsVerify"`
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	options := image.SearchOptions{
		Limit: query.Limit,
	}
	if _, found := r.URL.Query()["tlsVerify"]; found {
		options.InsecureSkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}

	if _, found := r.URL.Query()["filters"]; found {
		filter, err := image.ParseSearchFilter(query.Filters)
		if err != nil {
			utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse filters parameter for %s", r.URL.String()))
			return
		}
		options.Filter = *filter
	}

	searchResults, err := image.SearchImages(query.Term, options)
	if err != nil {
		utils.BadRequest(w, "term", query.Term, err)
		return
	}
	// Convert from image.SearchResults to entities.ImageSearchReport. We don't
	// want to leak any low-level packages into the remote client, which
	// requires converting.
	reports := make([]entities.ImageSearchReport, len(searchResults))
	for i := range searchResults {
		reports[i].Index = searchResults[i].Index
		reports[i].Name = searchResults[i].Name
		reports[i].Description = searchResults[i].Index
		reports[i].Stars = searchResults[i].Stars
		reports[i].Official = searchResults[i].Official
		reports[i].Automated = searchResults[i].Automated
	}

	utils.WriteResponse(w, http.StatusOK, reports)
}

// ImagesBatchRemove is the endpoint for batch image removal.
func ImagesBatchRemove(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		All    bool     `schema:"all"`
		Force  bool     `schema:"force"`
		Images []string `schema:"images"`
	}{
		All:   false,
		Force: false,
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	opts := entities.ImageRemoveOptions{All: query.All, Force: query.Force}

	imageEngine := abi.ImageEngine{Libpod: runtime}
	rmReport, rmErrors := imageEngine.Remove(r.Context(), query.Images, opts)

	strErrs := errorhandling.ErrorsToStrings(rmErrors)
	report := handlers.LibpodImagesRemoveReport{ImageRemoveReport: *rmReport, Errors: strErrs}
	utils.WriteResponse(w, http.StatusOK, report)
}

// ImagesRemove is the endpoint for removing one image.
func ImagesRemove(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	query := struct {
		Force bool `schema:"force"`
	}{
		Force: false,
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest,
			errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
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
		utils.Error(w, "error removing image", http.StatusNotFound, errorhandling.JoinErrors(rmErrors))
	case 2:
		// 409 - conflict error (in use by containers)
		utils.Error(w, "error removing image", http.StatusConflict, errorhandling.JoinErrors(rmErrors))
	default:
		// 500 - internal error
		utils.Error(w, "failed to remove image", http.StatusInternalServerError, errorhandling.JoinErrors(rmErrors))
	}
}
