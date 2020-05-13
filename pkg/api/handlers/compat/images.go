package compat

import (
	"encoding/json"
	"fmt"
	"io"
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
	"github.com/containers/libpod/pkg/auth"
	"github.com/containers/libpod/pkg/domain/entities"
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
		All     bool
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
	pruneCids, err := runtime.ImageRuntime().PruneImages(r.Context(), query.All, filters)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	for _, p := range pruneCids {
		idr = append(idr, types.ImageDeleteResponseItem{
			Deleted: p,
		})
	}

	//FIXME/TODO to do this exactly correct, pruneimages needs to return idrs and space-reclaimed, then we are golden
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
		Author    string `schema:"author"`
		Changes   string `schema:"changes"`
		Comment   string `schema:"comment"`
		Container string `schema:"container"`
		//fromSrc   string  # fromSrc is currently unused
		Pause bool   `schema:"pause"`
		Repo  string `schema:"repo"`
		Tag   string `schema:"tag"`
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
	sc := image2.GetSystemContext(rtc.Engine.SignaturePolicyPath, "", false)
	tag := "latest"
	options := libpod.ContainerCommitOptions{
		Pause: true,
	}
	options.CommitOptions = buildah.CommitOptions{
		SignaturePolicyPath:   rtc.Engine.SignaturePolicyPath,
		ReportWriter:          os.Stderr,
		SystemContext:         sc,
		PreferredManifestType: manifest.DockerV2Schema2MediaType,
	}

	input := handlers.CreateContainerConfig{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}

	if len(query.Tag) > 0 {
		tag = query.Tag
	}
	options.Message = query.Comment
	options.Author = query.Author
	options.Pause = query.Pause
	options.Changes = strings.Fields(query.Changes)
	ctr, err := runtime.LookupContainer(query.Container)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusNotFound, err)
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

func CreateImageFromSrc(w http.ResponseWriter, r *http.Request) {
	// 200 no error
	// 404 repo does not exist or no read access
	// 500 internal
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	query := struct {
		FromSrc string   `schema:"fromSrc"`
		Changes []string `schema:"changes"`
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}
	// fromSrc – Source to import. The value may be a URL from which the image can be retrieved or - to read the image from the request body. This parameter may only be used when importing an image.
	source := query.FromSrc
	if source == "-" {
		f, err := ioutil.TempFile("", "api_load.tar")
		if err != nil {
			utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to create tempfile"))
			return
		}
		source = f.Name()
		if err := SaveFromBody(f, r); err != nil {
			utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to write temporary file"))
		}
	}
	iid, err := runtime.Import(r.Context(), source, "", query.Changes, "", false)
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
		FromImage string `schema:"fromImage"`
		Tag       string `schema:"tag"`
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	fromImage := query.FromImage
	if len(query.Tag) >= 1 {
		fromImage = fmt.Sprintf("%s:%s", fromImage, query.Tag)
	}

	authConf, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse %q header for %s", auth.XRegistryAuthHeader, r.URL.String()))
		return
	}
	defer auth.RemoveAuthfile(authfile)

	registryOpts := image2.DockerRegistryOptions{DockerRegistryCreds: authConf}
	if sys := runtime.SystemContext(); sys != nil {
		registryOpts.DockerCertPath = sys.DockerCertPath
	}
	img, err := runtime.ImageRuntime().New(r.Context(),
		fromImage,
		"", // signature policy
		authfile,
		nil, // writer
		&registryOpts,
		image2.SigningOptions{},
		nil, // label
		util.PullImageMissing,
	)
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
	newImage, err := utils.GetImage(r, name)
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
	var summaries = make([]*entities.ImageSummary, len(images))
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

func LoadImages(w http.ResponseWriter, r *http.Request) {
	// TODO this is basically wrong
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)

	query := struct {
		Changes map[string]string `json:"changes"`
		Message string            `json:"message"`
		Quiet   bool              `json:"quiet"`
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "Failed to parse parameters for %s", r.URL.String()))
		return
	}

	var (
		err    error
		writer io.Writer
	)
	f, err := ioutil.TempFile("", "api_load.tar")
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to create tempfile"))
		return
	}
	if err := SaveFromBody(f, r); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to write temporary file"))
		return
	}
	id, err := runtime.LoadImage(r.Context(), "", f.Name(), writer, "")
	//id, err := runtime.Import(r.Context())
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to load image"))
		return
	}
	utils.WriteResponse(w, http.StatusOK, struct {
		Stream string `json:"stream"`
	}{
		Stream: fmt.Sprintf("Loaded image: %s\n", id),
	})
}
