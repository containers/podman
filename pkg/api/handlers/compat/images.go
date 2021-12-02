package compat

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/containers/buildah"
	"github.com/containers/common/libimage"
	"github.com/containers/common/pkg/config"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/pkg/shortnames"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/pkg/api/handlers"
	"github.com/containers/podman/v3/pkg/api/handlers/utils"
	api "github.com/containers/podman/v3/pkg/api/types"
	"github.com/containers/podman/v3/pkg/auth"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/domain/infra/abi"
	"github.com/containers/storage"
	"github.com/gorilla/schema"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// mergeNameAndTagOrDigest creates an image reference as string from the
// provided image name and tagOrDigest which can be a tag, a digest or empty.
func mergeNameAndTagOrDigest(name, tagOrDigest string) string {
	if len(tagOrDigest) == 0 {
		return name
	}

	separator := ":" // default to tag
	if _, err := digest.Parse(tagOrDigest); err == nil {
		// We have a digest, so let's change the separator.
		separator = "@"
	}
	return fmt.Sprintf("%s%s%s", name, separator, tagOrDigest)
}

func ExportImage(w http.ResponseWriter, r *http.Request) {
	// 200 ok
	// 500 server
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	tmpfile, err := ioutil.TempFile("", "api.tar")
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to create tempfile"))
		return
	}
	defer os.Remove(tmpfile.Name())

	name := utils.GetName(r)
	imageEngine := abi.ImageEngine{Libpod: runtime}

	saveOptions := entities.ImageSaveOptions{
		Format: "docker-archive",
		Output: tmpfile.Name(),
	}

	if err := imageEngine.Save(r.Context(), name, nil, saveOptions); err != nil {
		if errors.Cause(err) == storage.ErrImageUnknown {
			utils.ImageNotFound(w, name, errors.Wrapf(err, "failed to find image %s", name))
			return
		}
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to create tempfile"))
		return
	}

	if err := tmpfile.Close(); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to close tempfile"))
		return
	}

	rdr, err := os.Open(tmpfile.Name())
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to read the exported tarfile"))
		return
	}
	defer rdr.Close()
	utils.WriteResponse(w, http.StatusOK, rdr)
}

func CommitContainer(w http.ResponseWriter, r *http.Request) {
	var (
		destImage string
	)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		Author    string `schema:"author"`
		Changes   string `schema:"changes"`
		Comment   string `schema:"comment"`
		Container string `schema:"container"`
		// fromSrc   string  # fromSrc is currently unused
		Pause bool   `schema:"pause"`
		Repo  string `schema:"repo"`
		Tag   string `schema:"tag"`
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	rtc, err := runtime.GetConfig()
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Decode()"))
		return
	}
	sc := runtime.SystemContext()
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
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		Changes  []string `schema:"changes"`
		FromSrc  string   `schema:"fromSrc"`
		Message  string   `schema:"message"`
		Platform string   `schema:"platform"`
		Repo     string   `shchema:"repo"`
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
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

	platformSpecs := strings.Split(query.Platform, "/")
	opts := entities.ImageImportOptions{
		Source:    source,
		Changes:   query.Changes,
		Message:   query.Message,
		Reference: query.Repo,
		OS:        platformSpecs[0],
	}
	if len(platformSpecs) > 1 {
		opts.Architecture = platformSpecs[1]
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}
	report, err := imageEngine.Import(r.Context(), opts)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to import tarball"))
		return
	}
	// Success
	utils.WriteResponse(w, http.StatusOK, struct {
		Status         string            `json:"status"`
		Progress       string            `json:"progress"`
		ProgressDetail map[string]string `json:"progressDetail"`
		Id             string            `json:"id"` // nolint
	}{
		Status:         report.Id,
		ProgressDetail: map[string]string{},
		Id:             report.Id,
	})
}

type pullResult struct {
	images []*libimage.Image
	err    error
}

func CreateImageFromImage(w http.ResponseWriter, r *http.Request) {
	// 200 no error
	// 404 repo does not exist or no read access
	// 500 internal
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		FromImage string `schema:"fromImage"`
		Tag       string `schema:"tag"`
		Platform  string `schema:"platform"`
	}{
		// This is where you can override the golang default value for one of fields
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	fromImage := mergeNameAndTagOrDigest(query.FromImage, query.Tag)

	// without this early check this function would return 200 but reported error via body stream soon after
	// it's better to let caller know early via HTTP status code that request cannot be processed
	_, err := shortnames.Resolve(runtime.SystemContext(), fromImage)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrap(err, "failed to resolve image name"))
		return
	}

	authConf, authfile, key, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, "failed to retrieve repository credentials", http.StatusBadRequest, errors.Wrapf(err, "failed to parse %q header for %s", key, r.URL.String()))
		return
	}
	defer auth.RemoveAuthfile(authfile)

	pullOptions := &libimage.PullOptions{}
	pullOptions.AuthFilePath = authfile
	if authConf != nil {
		pullOptions.Username = authConf.Username
		pullOptions.Password = authConf.Password
		pullOptions.IdentityToken = authConf.IdentityToken
	}
	pullOptions.Writer = os.Stderr // allows for debugging on the server

	// Handle the platform.
	platformSpecs := strings.Split(query.Platform, "/")
	pullOptions.OS = platformSpecs[0] // may be empty
	if len(platformSpecs) > 1 {
		pullOptions.Architecture = platformSpecs[1]
		if len(platformSpecs) > 2 {
			pullOptions.Variant = platformSpecs[2]
		}
	}

	progress := make(chan types.ProgressProperties)
	pullOptions.Progress = progress

	pullResChan := make(chan pullResult)
	go func() {
		pulledImages, err := runtime.LibimageRuntime().Pull(r.Context(), fromImage, config.PullPolicyAlways, pullOptions)
		pullResChan <- pullResult{images: pulledImages, err: err}
	}()

	flush := func() {
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	flush()

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)

loop: // break out of for/select infinite loop
	for {
		var report struct {
			Stream   string `json:"stream,omitempty"`
			Status   string `json:"status,omitempty"`
			Progress struct {
				Current uint64 `json:"current,omitempty"`
				Total   int64  `json:"total,omitempty"`
			} `json:"progressDetail,omitempty"`
			Error string `json:"error,omitempty"`
			Id    string `json:"id,omitempty"` // nolint
		}
		select {
		case e := <-progress:
			switch e.Event {
			case types.ProgressEventNewArtifact:
				report.Status = "Pulling fs layer"
			case types.ProgressEventRead:
				report.Status = "Downloading"
				report.Progress.Current = e.Offset
				report.Progress.Total = e.Artifact.Size
			case types.ProgressEventSkipped:
				report.Status = "Already exists"
			case types.ProgressEventDone:
				report.Status = "Download complete"
			}
			report.Id = e.Artifact.Digest.Encoded()[0:12]
			if err := enc.Encode(report); err != nil {
				logrus.Warnf("Failed to json encode error %q", err.Error())
			}
			flush()
		case pullRes := <-pullResChan:
			err := pullRes.err
			pulledImages := pullRes.images
			if err != nil {
				report.Error = err.Error()
			} else {
				if len(pulledImages) > 0 {
					img := pulledImages[0].ID()
					if utils.IsLibpodRequest(r) {
						report.Status = "Pull complete"
					} else {
						report.Status = "Download complete"
					}
					report.Id = img[0:12]
				} else {
					report.Error = "internal error: no images pulled"
				}
			}
			if err := enc.Encode(report); err != nil {
				logrus.Warnf("Failed to json encode error %q", err.Error())
			}
			flush()
			break loop // break out of for/select infinite loop
		}
	}
}

func GetImage(w http.ResponseWriter, r *http.Request) {
	// 200 no error
	// 404 no such
	// 500 internal
	name := utils.GetName(r)
	newImage, err := utils.GetImage(r, name)
	if err != nil {
		// Here we need to fiddle with the error message because docker-py is looking for "No
		// such image" to determine on how to raise the correct exception.
		errMsg := strings.ReplaceAll(err.Error(), "image not known", "No such image")
		utils.Error(w, "Something went wrong.", http.StatusNotFound, errors.Errorf("failed to find image %s: %s", name, errMsg))
		return
	}
	inspect, err := handlers.ImageDataToImageInspect(r.Context(), newImage)
	if err != nil {
		utils.Error(w, "Server error", http.StatusInternalServerError, errors.Wrapf(err, "failed to convert ImageData to ImageInspect '%s'", inspect.ID))
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

	summaries := make([]*entities.ImageSummary, 0, len(images))
	for _, img := range images {
		// If the image is a manifest list, extract as much as we can.
		if isML, _ := img.IsManifestList(r.Context()); isML {
			continue
		}

		is, err := handlers.ImageToImageSummary(img)
		if err != nil {
			utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "Failed transform image summaries"))
			return
		}
		summaries = append(summaries, is)
	}
	utils.WriteResponse(w, http.StatusOK, summaries)
}

func LoadImages(w http.ResponseWriter, r *http.Request) {
	// TODO this is basically wrong
	// TODO ... improve these ^ messages to something useful
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		Changes map[string]string `json:"changes"` // Ignored
		Message string            `json:"message"` // Ignored
		Quiet   bool              `json:"quiet"`   // Ignored
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}

	// First write the body to a temporary file that we can later attempt
	// to load.
	f, err := ioutil.TempFile("", "api_load.tar")
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to create tempfile"))
		return
	}
	defer func() {
		err := os.Remove(f.Name())
		if err != nil {
			logrus.Errorf("Failed to remove temporary file: %v.", err)
		}
	}()
	if err := SaveFromBody(f, r); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to write temporary file"))
		return
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}

	loadOptions := entities.ImageLoadOptions{Input: f.Name()}
	loadReport, err := imageEngine.Load(r.Context(), loadOptions)
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to load image"))
		return
	}

	if len(loadReport.Names) != 1 {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Errorf("%d instead of 1 were loaded", len(loadReport.Names)))
		return
	}

	utils.WriteResponse(w, http.StatusOK, struct {
		Stream string `json:"stream"`
	}{
		Stream: fmt.Sprintf("Loaded image: %s\n", loadReport.Names[0]),
	})
}

func ExportImages(w http.ResponseWriter, r *http.Request) {
	// 200 OK
	// 500 Error
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		Names []string `schema:"names"`
	}{
		// This is where you can override the golang default value for one of fields
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, errors.Wrapf(err, "failed to parse parameters for %s", r.URL.String()))
		return
	}
	if len(query.Names) <= 0 {
		utils.Error(w, "Something went wrong.", http.StatusBadRequest, fmt.Errorf("no images to download"))
		return
	}

	images := query.Names
	tmpfile, err := ioutil.TempFile("", "api.tar")
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to create tempfile"))
		return
	}
	defer os.Remove(tmpfile.Name())
	if err := tmpfile.Close(); err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "unable to close tempfile"))
		return
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}

	saveOptions := entities.ImageSaveOptions{Format: "docker-archive", Output: tmpfile.Name(), MultiImageArchive: true}
	if err := imageEngine.Save(r.Context(), images[0], images[1:], saveOptions); err != nil {
		utils.InternalServerError(w, err)
		return
	}

	rdr, err := os.Open(tmpfile.Name())
	if err != nil {
		utils.Error(w, "Something went wrong.", http.StatusInternalServerError, errors.Wrap(err, "failed to read the exported tarfile"))
		return
	}
	defer rdr.Close()
	utils.WriteResponse(w, http.StatusOK, rdr)
}
