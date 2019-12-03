package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/containers/buildah"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/libpod/libpod"
	image2 "github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/storage"
	"github.com/docker/docker/api/types"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func saveFromBody(f *os.File, r *http.Request) error { // nolint
	if _, err := io.Copy(f, r.Body); err != nil {
		return err
	}
	return f.Close()
}

func LoadImage(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	var (
		err    error
		writer io.Writer
	)
	quiet := false
	if len(r.Form.Get("quiet")) > 0 {
		quiet, err = strconv.ParseBool(r.Form.Get("quiet"))
		if err != nil {
			Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "bad value for quiet"))
			return
		}
	}
	f, err := ioutil.TempFile("", "api_load.tar")
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to create tempfile"))
		return
	}
	if err := saveFromBody(f, r); err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to write temporary file"))
		return
	}
	id, err := runtime.LoadImage(r.Context(), "", f.Name(), writer, "")
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to load image"))
		return
	}
	_ = quiet
	WriteResponse(w, http.StatusOK, struct {
		Stream string `json:"stream"`
	}{
		Stream: fmt.Sprintf("Loaded image: %s\n", id),
	})
}

func ExportImage(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	name := mux.Vars(r)["name"]
	newImage, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		ImageNotFound(w, name, errors.Wrapf(err, "Failed to find image %s", name))
		return
	}
	tmpfile, err := ioutil.TempFile("", "api.tar")
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to create tempfile"))
		return
	}
	if err := tmpfile.Close(); err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to close tempfile"))
		return
	}
	if err := newImage.Save(r.Context(), name, "docker-archive", tmpfile.Name(), []string{}, false, false); err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to save image"))
		return
	}
	rdr, err := os.Open(tmpfile.Name())
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to read the exported tarfile"))
		return
	}
	WriteResponse(w, http.StatusOK, rdr)
}

func PruneImages(w http.ResponseWriter, r *http.Request) {
	var (
		runtime       = r.Context().Value("runtime").(*libpod.Runtime)
		dangling bool = true
		err      error
	)
	filters := make(map[string][]string)
	if len(r.Form.Get("filters")) > 0 {
		if err := json.Unmarshal([]byte(r.Form.Get("filters")), &filters); err != nil {
			Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "marshalling filters"))
			return
		}
	}
	// until ts is not supported on podman prune
	if len(filters["until"]) > 0 {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "until is not supported yet"))
		return
	}
	// labels are not supported on podman prune
	if len(filters["label"]) > 0 {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "labelis not supported yet"))
		return
	}

	if len(filters["dangling"]) > 0 {
		dangling, err = strconv.ParseBool(filters["dangling"][0])
		if err != nil {
			Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "processing dangling filter"))
			return
		}
	}
	idr := []types.ImageDeleteResponseItem{}
	//
	// This code needs to be migrated to libpod to work correctly.  I could not
	// work my around the information docker needs with the existing prune in libpod.
	//
	pruneImages, err := runtime.ImageRuntime().GetPruneImages(!dangling, []image2.ImageFilter{})
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to get images to prune"))
		return
	}
	for _, p := range pruneImages {
		repotags, err := p.RepoTags()
		if err != nil {
			Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to get repotags for image"))
			return
		}
		if err := p.Remove(r.Context(), true); err != nil {
			if errors.Cause(err) == storage.ErrImageUsedByContainer {
				logrus.Warnf("Failed to prune image %s as it is in use: %v", p.ID(), err)
				continue
			}
			Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to prune image"))
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
	WriteResponse(w, http.StatusOK, ImagesPruneReport{ipr})
}

func CommitContainer(w http.ResponseWriter, r *http.Request) {
	var (
		runtime   = r.Context().Value("runtime").(*libpod.Runtime)
		err       error
		destImage string
	)
	rtc, err := runtime.GetConfig()
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}
	sc := image2.GetSystemContext(rtc.SignaturePolicyPath, "", false)
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

	input := CreateContainerConfig{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}

	nameOrID := r.Form.Get("container")
	repo := r.Form.Get("repo")
	if len(r.Form.Get("tag")) > 0 {
		tag = r.Form.Get("tag")
	}
	options.Message = r.Form.Get("comment")
	options.Author = r.Form.Get("author")
	if len(r.Form.Get("pause")) > 0 {
		options.Pause, err = strconv.ParseBool(r.Form.Get("pause"))
		if err != nil {
			Error(w, "Something went wrong.", http.StatusInternalServerError, err)
			return
		}
	}
	if len(r.Form.Get("changes")) > 0 {
		options.Changes = r.Form["changes"]
	}
	ctr, err := runtime.LookupContainer(nameOrID)
	if err != nil {
		Error(w, "Something went wrong.", http.StatusNotFound, err)
		return
	}

	// I know mitr hates this ... but doing for now
	if len(repo) > 1 {
		destImage = fmt.Sprintf("%s:%s", repo, tag)
	}

	commitImage, err := ctr.Commit(r.Context(), destImage, options)
	if err != nil && !strings.Contains(err.Error(), "is not running") {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "CommitFailure"))
		return
	}
	WriteResponse(w, http.StatusOK, IDResponse{ID: commitImage.ID()}) // nolint
}

func CreateImageFromSrc(w http.ResponseWriter, r *http.Request) {
	// fromSrc – Source to import. The value may be a URL from which the image can be retrieved or - to read the image from the request body. This parameter may only be used when importing an image.
	var (
		runtime = r.Context().Value("runtime").(*libpod.Runtime)
		changes []string
	)
	fromSrc := r.Form.Get("fromSrc")
	source := fromSrc
	if fromSrc == "-" {
		f, err := ioutil.TempFile("", "api_load.tar")
		if err != nil {
			Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to create tempfile"))
			return
		}
		source = f.Name()
		if err := saveFromBody(f, r); err != nil {
			Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to write temporary file"))
		}
	}
	if len(r.Form["changes"]) > 0 {
		changes = r.Form["changes"]
	}
	iid, err := runtime.Import(r.Context(), source, "", changes, "", false)
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to import tarball"))
		return
	}
	tmpfile, err := ioutil.TempFile("", "fromsrc.tar")
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to create tempfile"))
		return
	}
	if err := tmpfile.Close(); err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to close tempfile"))
		return
	}
	// Success
	WriteResponse(w, http.StatusOK, struct {
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

func CreateImageFromImage(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	/*
	   fromImage – Name of the image to pull. The name may include a tag or digest. This parameter may only be used when pulling an image. The pull is cancelled if the HTTP connection is closed.
	   repo – Repository name given to an image when it is imported. The repo may include a tag. This parameter may only be used when importing an image.
	   tag – Tag or digest. If empty when pulling an image, this causes all tags for the given image to be pulled.
	*/
	fromImage := r.Form.Get("fromImage")

	tag := r.Form.Get("tag")
	if tag != "" {
		fromImage = fmt.Sprintf("%s:%s", fromImage, tag)
	}

	// TODO
	// We are eating the output right now because we haven't talked about how to deal with multiple responses yet
	img, err := runtime.ImageRuntime().New(r.Context(), fromImage, "", "", nil, &image2.DockerRegistryOptions{}, image2.SigningOptions{}, nil, util.PullImageMissing)
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}

	// Success
	WriteResponse(w, http.StatusOK, struct {
		Status         string            `json:"status"`
		Error          string            `json:"error"`
		Progress       string            `json:"progress"`
		ProgressDetail map[string]string `json:"progressDetail"`
		Id             string            `json:"id"`
	}{
		Status:         fmt.Sprintf("pulling image (%s) from %s", img.Tag, strings.Join(img.Names(), ", ")),
		ProgressDetail: map[string]string{},
		Id:             img.ID(),
	})
}

func TagImage(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	// /v1.xx/images/(name)/tag
	name := mux.Vars(r)["name"]
	newImage, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		ImageNotFound(w, name, errors.Wrapf(err, "Failed to find image %s", name))
		return
	}
	tag := "latest"
	if len(r.Form.Get("tag")) > 0 {
		tag = r.Form.Get("tag")
	}
	if len(r.Form.Get("repo")) < 1 {
		Error(w, "Something went wrong.", http.StatusBadRequest, errors.New("repo parameter is required to tag an image"))
		return
	}
	repo := r.Form.Get("repo")
	tagName := fmt.Sprintf("%s:%s", repo, tag)
	if err := newImage.TagImage(tagName); err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	WriteResponse(w, http.StatusCreated, "")
}

func RemoveImage(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	name := mux.Vars(r)["name"]
	newImage, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		ImageNotFound(w, name, errors.Wrapf(err, "Failed to find image %s", name))
		return
	}

	force := false
	if len(r.Form.Get("force")) > 0 {
		force, err = strconv.ParseBool(r.Form.Get("force"))
		if err != nil {
			Error(w, "Something went wrong.", http.StatusBadRequest, err)
			return
		}
	}
	_, err = runtime.RemoveImage(r.Context(), newImage, force)
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}
	// TODO
	// This will need to be fixed for proper response, like Deleted: and Untagged:
	m := make(map[string]string)
	m["Deleted"] = newImage.ID()
	foo := []map[string]string{}
	foo = append(foo, m)
	WriteResponse(w, http.StatusOK, foo)

}
func GetImage(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := mux.Vars(r)["name"]

	newImage, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		Error(w, "Something went wrong.", http.StatusNotFound, errors.Wrapf(err, "Failed to find image %s", name))
		return
	}

	inspect, err := ImageDataToImageInspect(r.Context(), newImage)
	if err != nil {
		Error(w, "Server error", http.StatusInternalServerError, errors.Wrapf(err, "Failed to convert ImageData to ImageInspect '%s'", inspect.ID))
		return
	}
	WriteResponse(w, http.StatusOK, inspect)
}

func GetImages(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	query := struct {
		all     bool
		filters string
		digests bool
	}{
		// This is where you can override the golang default value for one of fields
	}

	var err error
	t := r.Form.Get("all")
	if t != "" {
		query.all, err = strconv.ParseBool(t)
		if err != nil {
			Error(w, "Server error", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse 'all' parameter %s", t))
			return
		}
	}

	// TODO How do we want to process filters
	t = r.Form.Get("filters")
	if t != "" {
		query.filters = t
	}

	t = r.Form.Get("digests")
	if t != "" {
		query.digests, err = strconv.ParseBool(t)
		if err != nil {
			Error(w, "Server error", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse 'digests' parameter %s", t))
			return
		}
	}

	// FIXME placeholder until filters are implemented
	_ = query

	images, err := runtime.ImageRuntime().GetImages()
	if err != nil {
		Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "Failed to obtain the list of images from storage"))
		return
	}

	var summaries = make([]*ImageSummary, len(images))
	for j, img := range images {
		is, err := ImageToImageSummary(img)
		if err != nil {
			Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrapf(err, "Failed to convert storage image '%s' to API image", img.ID()))
			return
		}
		summaries[j] = is
	}

	WriteResponse(w, http.StatusOK, summaries)
}
func ImageExists(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	name := mux.Vars(r)["name"]

	_, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		Error(w, "Something went wrong.", http.StatusNotFound, errors.Wrapf(err, "Failed to find image %s", name))
		return
	}
	WriteResponse(w, http.StatusOK, "Ok")
}
