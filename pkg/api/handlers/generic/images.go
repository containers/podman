package generic

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/containers/buildah"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/libpod/libpod"
	image2 "github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/containers/libpod/pkg/util"
	"github.com/docker/docker/api/types"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

func ExportImage(w http.ResponseWriter, r *http.Request) {
	// 200 ok
	// 500 server
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	name := utils.GetName(r)
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
	var (
		filters []string
	)
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	query := struct {
		Filters map[string][]string `schema:"filters"`
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	idr := []types.ImageDeleteResponseItem{}
	for k, v := range query.Filters {
		for _, val := range v {
			filters = append(filters, fmt.Sprintf("%s=%s", k, val))
		}
	}
	pruneCids, err := runtime.ImageRuntime().PruneImages(r.Context(), false, filters)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	for _, p := range pruneCids {
		idr = append(idr, types.ImageDeleteResponseItem{
			Deleted: p,
		})
	}

	//FIXME/TODO to do this exacty correct, pruneimages needs to return idrs and space-reclaimed, then we are golden
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

func CreateImageFromImage(w http.ResponseWriter, r *http.Request) {
	// 200 no error
	// 404 repo does not exist or no read access
	// 500 internal
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	query := struct {
		fromImage string
		tag       string
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	/*
	   fromImage – Name of the image to pull. The name may include a tag or digest. This parameter may only be used when pulling an image. The pull is cancelled if the HTTP connection is closed.
	   repo – Repository name given to an image when it is imported. The repo may include a tag. This parameter may only be used when importing an image.
	   tag – Tag or digest. If empty when pulling an image, this causes all tags for the given image to be pulled.
	*/
	fromImage := query.fromImage
	if len(query.tag) < 1 {
		fromImage = fmt.Sprintf("%s:%s", fromImage, query.tag)
	}

	// TODO
	// We are eating the output right now because we haven't talked about how to deal with multiple responses yet
	img, err := runtime.ImageRuntime().New(r.Context(), fromImage, "", "", nil, &image2.DockerRegistryOptions{}, image2.SigningOptions{}, nil, util.PullImageMissing)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, err)
		return
	}

	// Success
	utils.WriteResponse(w, http.StatusOK, struct {
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

func GetImage(w http.ResponseWriter, r *http.Request) {
	// 200 no error
	// 404 no such
	// 500 internal
	name := utils.GetName(r)
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
	images, err := utils.GetImages(w, r)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Failed get images"))
		return
	}
	var summaries = make([]*handlers.ImageSummary, len(images)+1)
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
