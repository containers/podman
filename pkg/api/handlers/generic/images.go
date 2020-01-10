package generic

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/containers/buildah"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/manifest"
	imageTypes "github.com/containers/image/v5/types"
	"github.com/containers/libpod/libpod"
	libpodImage "github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/storage"
	"github.com/docker/docker/api/types"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func ExportImage(w http.ResponseWriter, r *http.Request) {
	// 200 ok
	// 500 server
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	name := mux.Vars(r)["name"]
	newImage, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		utils.ImageNotFound(w, name, errors.Wrapf(err, "Failed to find image %s", name))
		return
	}
	tmpfile, err := ioutil.TempFile("", "api.tar")
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to create tempfile"))
		return
	}
	if err := tmpfile.Close(); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to close tempfile"))
		return
	}
	if err := newImage.Save(r.Context(), name, "docker-archive", tmpfile.Name(), []string{}, false, false); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to save image"))
		return
	}
	rdr, err := os.Open(tmpfile.Name())
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to read the exported tarfile"))
		return
	}
	defer rdr.Close()
	defer os.Remove(tmpfile.Name())
	utils.WriteResponse(w, http.StatusOK, rdr)
}

func PruneImages(w http.ResponseWriter, r *http.Request) {
	// 200 no error
	// 500 internal
	var (
		dangling bool = true
		err      error
	)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	query := struct {
		filters map[string]string
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	// FIXME This is likely wrong due to it not being a map[string][]string

	// until ts is not supported on podman prune
	if len(query.filters["until"]) > 0 {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "until is not supported yet"))
		return
	}
	// labels are not supported on podman prune
	if len(query.filters["label"]) > 0 {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "labelis not supported yet"))
		return
	}

	if len(query.filters["dangling"]) > 0 {
		dangling, err = strconv.ParseBool(query.filters["dangling"])
		if err != nil {
			utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "processing dangling filter"))
			return
		}
	}
	idr := []types.ImageDeleteResponseItem{}
	//
	// This code needs to be migrated to libpod to work correctly.  I could not
	// work my around the information docker needs with the existing prune in libpod.
	//
	pruneImages, err := runtime.ImageRuntime().GetPruneImages(!dangling, []libpodImage.ImageFilter{})
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to get images to prune"))
		return
	}
	for _, p := range pruneImages {
		repotags, err := p.RepoTags()
		if err != nil {
			utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to get repotags for image"))
			return
		}
		if err := p.Remove(r.Context(), true); err != nil {
			if errors.Cause(err) == storage.ErrImageUsedByContainer {
				logrus.Warnf("Failed to prune image %s as it is in use: %v", p.ID(), err)
				continue
			}
			utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to prune image"))
			return
		}
		// newimageevent is not export therefore we cannot record the event. this will be fixed
		// when the prune is fixed in libpod
		// defer p.newImageEvent(events.Prune)
		response := types.ImageDeleteResponseItem{
			Deleted: fmt.Sprintf("sha256:%s", p.ID()), // I ack this is not ideal
		}
		if len(repotags) > 0 {
			response.Untagged = repotags[0]
		}
		idr = append(idr, response)
	}
	ipr := types.ImagesPruneReport{
		ImagesDeleted:  idr,
		SpaceReclaimed: 1, // TODO we cannot supply this right now
	}
	utils.WriteResponse(w, http.StatusOK, handlers.ImagesPruneReport{ImagesPruneReport: ipr})
}

func CommitContainer(w http.ResponseWriter, r *http.Request) {
	var (
		destImage string
	)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	query := struct {
		author    string
		changes   string
		comment   string
		container string
		//fromSrc   string  # fromSrc is currently unused
		pause bool
		repo  string
		tag   string
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	rtc, err := runtime.GetConfig()
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}
	sc := libpodImage.GetSystemContext(rtc.SignaturePolicyPath, "", false)
	tag := "latest"
	options := libpod.ContainerCommitOptions{
		Pause: true,
	}
	options.CommitOptions = buildah.CommitOptions{
		SignaturePolicyPath:   rtc.SignaturePolicyPath,
		ReportWriter:          os.Stderr,
		SystemContext:         sc,
		PreferredManifestType: manifest.DockerV2Schema2MediaType,
	}

	input := handlers.CreateContainerConfig{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}

	if len(query.tag) > 0 {
		tag = query.tag
	}
	options.Message = query.comment
	options.Author = query.author
	options.Pause = query.pause
	options.Changes = strings.Fields(query.changes)
	ctr, err := runtime.LookupContainer(query.container)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusNotFound, err)
		return
	}

	// I know mitr hates this ... but doing for now
	if len(query.repo) > 1 {
		destImage = fmt.Sprintf("%s:%s", query.repo, tag)
	}

	commitImage, err := ctr.Commit(r.Context(), destImage, options)
	if err != nil && !strings.Contains(err.Error(), "is not running") {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "CommitFailure"))
		return
	}
	utils.WriteResponse(w, http.StatusOK, handlers.IDResponse{ID: commitImage.ID()}) // nolint
}

func CreateImageFromSrc(w http.ResponseWriter, r *http.Request) {
	// 200 no error
	// 404 repo does not exist or no read access
	// 500 internal
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	query := struct {
		fromSrc string
		changes []string
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	// fromSrc – Source to import. The value may be a URL from which the image can be retrieved or - to read the image from the request body. This parameter may only be used when importing an image.
	source := query.fromSrc
	if source == "-" {
		f, err := ioutil.TempFile("", "api_load.tar")
		if err != nil {
			utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to create tempfile"))
			return
		}
		source = f.Name()
		if err := handlers.SaveFromBody(f, r); err != nil {
			utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to write temporary file"))
		}
	}
	iid, err := runtime.Import(r.Context(), source, "", query.changes, "", false)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to import tarball"))
		return
	}
	tmpfile, err := ioutil.TempFile("", "fromsrc.tar")
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to create tempfile"))
		return
	}
	if err := tmpfile.Close(); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to close tempfile"))
		return
	}
	// Success
	utils.WriteResponse(w, http.StatusOK, struct {
		Status         string            `json:"status"`
		Progress       string            `json:"progress"`
		ProgressDetail map[string]string `json:"progressDetail"`
		Id             string            `json:"id"`
	}{
		Status:         iid,
		ProgressDetail: map[string]string{},
		Id:             iid,
	})

}

// https://docs.docker.com/engine/api/v1.40/#operation/ImageCreate
type imagesCreateQuery struct {
	// Name of the image to pull. The name may include a tag or digest.
	// This parameter may only be used when pulling an image.  The pull is
	// cancelled if the HTTP connection is closed.
	fromImage string
	// Source to import. The value may be a URL from which the image can be
	// retrieved or - to read the image from the request body. This
	// parameter may only be used when importing an image.
	fromSrc string
	// Repository name given to an image when it is imported. The repo may
	// include a tag. This parameter may only be used when importing an
	// image.
	repo string
	// Tag or digest. If empty when pulling an image, this causes all tags
	// for the given image to be pulled.
	tag string
	// Platform in the format os[/arch[/variant]]
	platform string
	// Set commit message for imported image.
	message string
}

func CreateImageFromImage(w http.ResponseWriter, r *http.Request) {
	// 200 no error
	// 404 repo does not exist or no read access
	// 500 internal
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	query := imagesCreateQuery{
		fromImage: r.Form.Get("fromImage"),
		fromSrc:   r.Form.Get("fromSrc"),
		repo:      r.Form.Get("repo"),
		tag:       r.Form.Get("tag"),
		platform:  r.Form.Get("platform"),
		message:   r.Form.Get("message"),
	}

	// We do not support importing images yet.
	if query.fromImage == "" {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Errorf("importing images is not yet supported"))
		return
	}

	if query.message != "" {
		logrus.Info("Ignoring /images/create `message` argument")
	}

	// Assemble the image reference.
	ref, err := reference.Parse(query.fromImage)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "error parsing `fromImage`: %q", query.fromImage))
		return
	}
	// query.fromImage might be tagged already, so we need to check if need to
	// append `query.tag`.
	image := ref.String()
	_, isTagged := ref.(reference.Tagged)
	if !isTagged && query.tag != "" {
		image += ":" + query.tag
	}

	// Assemble the registry options.
	authEncoded := r.Header.Get("X-Registry-Auth")
	authConfig := &imageTypes.DockerAuthConfig{}
	if authEncoded != "" {
		authJSON := base64.NewDecoder(base64.URLEncoding, strings.NewReader(authEncoded))
		if err := json.NewDecoder(authJSON).Decode(authConfig); err != nil {
			// for a pull it is not an error if no auth was given
			// to increase compatibility with the existing api it is defaulting to be empty
			authConfig = nil
		}
	}
	registryOptions := &libpodImage.DockerRegistryOptions{
		// TODO(docker-py) - the docker-py tests are checking for "unknown
		// operating system" and "invalid platform" strings in the error
		// message, which containers/image does NOT provide.  Shall we change
		// the error wording in containers/image or accept it as a sad fact of
		// life?
		OSChoice:            query.platform,
		DockerRegistryCreds: authConfig,
	}

	// TODO
	// We are eating the output right now because we haven't talked about how to deal with multiple responses yet
	img, err := runtime.ImageRuntime().New(r.Context(), image, "", "", nil, registryOptions, libpodImage.SigningOptions{}, nil, util.PullImageMissing)
	if err != nil {
		// TODO - we need to map `err` to the corresponding http status code.
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}

	// Success
	utils.WriteResponse(w, http.StatusOK, struct {
		Status         string            `json:"status"`
		Error          string            `json:"error"`
		Progress       string            `json:"progress"`
		ProgressDetail map[string]string `json:"progressDetail"`
		ID             string            `json:"id"`
	}{
		Status:         fmt.Sprintf("pulling image (%s) from %s", img.Tag, strings.Join(img.Names(), ", ")),
		ProgressDetail: map[string]string{},
		ID:             img.ID(),
	})
}

func GetImage(w http.ResponseWriter, r *http.Request) {
	// 200 no error
	// 404 no such
	// 500 internal
	name := mux.Vars(r)["name"]
	newImage, err := handlers.GetImage(r, name)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusNotFound, errors.Wrapf(err, "Failed to find image %s", name))
		return
	}
	inspect, err := handlers.ImageDataToImageInspect(r.Context(), newImage)
	if err != nil {
		utils.Error(w, "Server error", http.StatusInternalServerError, errors.Wrapf(err, "Failed to convert ImageData to ImageInspect '%s'", inspect.ID))
		return
	}
	utils.WriteResponse(w, http.StatusOK, inspect)
}

func GetImages(w http.ResponseWriter, r *http.Request) {
	// 200 ok
	// 500 internal
	images, err := utils.GetImages(w, r)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Failed get images"))
		return
	}
	var summaries = make([]*handlers.ImageSummary, len(images))
	for j, img := range images {
		is, err := handlers.ImageToImageSummary(img)
		if err != nil {
			utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Failed transform image summaries"))
			return
		}
		summaries[j] = is
	}
	utils.WriteResponse(w, http.StatusOK, summaries)
}
