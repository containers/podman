//go:build !remote

package compat

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/containers/buildah"
	"github.com/containers/common/libimage"
	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/filters"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/pkg/api/handlers"
	"github.com/containers/podman/v5/pkg/api/handlers/utils"
	api "github.com/containers/podman/v5/pkg/api/types"
	"github.com/containers/podman/v5/pkg/auth"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/domain/infra/abi"
	"github.com/containers/podman/v5/pkg/util"
	"github.com/containers/storage"
	docker "github.com/docker/docker/api/types"
	dockerContainer "github.com/docker/docker/api/types/container"
	dockerImage "github.com/docker/docker/api/types/image"
	"github.com/docker/go-connections/nat"
	"github.com/opencontainers/go-digest"
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

	tmpfile, err := os.CreateTemp("", "api.tar")
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("unable to create tempfile: %w", err))
		return
	}
	defer os.Remove(tmpfile.Name())

	name := utils.GetName(r)
	possiblyNormalizedName, err := utils.NormalizeToDockerHub(r, name)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("normalizing image: %w", err))
		return
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}

	saveOptions := entities.ImageSaveOptions{
		Format: "docker-archive",
		Output: tmpfile.Name(),
	}

	if err := imageEngine.Save(r.Context(), possiblyNormalizedName, nil, saveOptions); err != nil {
		if errors.Is(err, storage.ErrImageUnknown) {
			utils.ImageNotFound(w, name, fmt.Errorf("failed to find image %s: %w", name, err))
			return
		}
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("unable to create tempfile: %w", err))
		return
	}

	if err := tmpfile.Close(); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("unable to close tempfile: %w", err))
		return
	}

	rdr, err := os.Open(tmpfile.Name())
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to read the exported tarfile: %w", err))
		return
	}
	defer rdr.Close()
	utils.WriteResponse(w, http.StatusOK, rdr)
}

func CommitContainer(w http.ResponseWriter, r *http.Request) {
	decoder := utils.GetDecoder(r)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		Author    string   `schema:"author"`
		Changes   []string `schema:"changes"`
		Comment   string   `schema:"comment"`
		Container string   `schema:"container"`
		Pause     bool     `schema:"pause"`
		Squash    bool     `schema:"squash"`
		Repo      string   `schema:"repo"`
		Tag       string   `schema:"tag"`
		// fromSrc   string  # fromSrc is currently unused
	}{
		Tag: "latest",
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	rtc, err := runtime.GetConfig()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("Decode(): %w", err))
		return
	}
	sc := runtime.SystemContext()
	options := libpod.ContainerCommitOptions{
		Pause: true,
	}
	options.CommitOptions = buildah.CommitOptions{
		SignaturePolicyPath:   rtc.Engine.SignaturePolicyPath,
		ReportWriter:          os.Stderr,
		SystemContext:         sc,
		PreferredManifestType: manifest.DockerV2Schema2MediaType,
	}

	options.Message = query.Comment
	options.Author = query.Author
	options.Pause = query.Pause
	options.Squash = query.Squash
	options.Changes = util.DecodeChanges(query.Changes)
	if r.Body != nil {
		defer r.Body.Close()
		if options.CommitOptions.OverrideConfig, err = abi.DecodeOverrideConfig(r.Body); err != nil {
			utils.Error(w, http.StatusBadRequest, err)
			return
		}
	}
	ctr, err := runtime.LookupContainer(query.Container)
	if err != nil {
		utils.Error(w, http.StatusNotFound, err)
		return
	}

	var destImage string
	if len(query.Repo) > 1 {
		destImage = fmt.Sprintf("%s:%s", query.Repo, query.Tag)
		possiblyNormalizedName, err := utils.NormalizeToDockerHub(r, destImage)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, fmt.Errorf("normalizing image: %w", err))
			return
		}
		destImage = possiblyNormalizedName
	}

	commitImage, err := ctr.Commit(r.Context(), destImage, options)
	if err != nil && !strings.Contains(err.Error(), "is not running") {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusCreated, entities.IDResponse{ID: commitImage.ID()})
}

func CreateImageFromSrc(w http.ResponseWriter, r *http.Request) {
	// 200 no error
	// 404 repo does not exist or no read access
	// 500 internal
	decoder := utils.GetDecoder(r)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		Changes  []string `schema:"changes"`
		FromSrc  string   `schema:"fromSrc"`
		Message  string   `schema:"message"`
		Platform string   `schema:"platform"`
		Repo     string   `schema:"repo"`
		Tag      string   `schema:"tag"`
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	// fromSrc – Source to import. The value may be a URL from which the image can be retrieved or - to read the image from the request body. This parameter may only be used when importing an image.
	source := query.FromSrc
	if source == "-" {
		f, err := os.CreateTemp("", "api_load.tar")
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to create tempfile: %w", err))
			return
		}

		source = f.Name()
		if err := SaveFromBody(f, r); err != nil {
			utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to write temporary file: %w", err))
			return
		}
	}

	reference := query.Repo
	if query.Repo != "" {
		possiblyNormalizedName, err := utils.NormalizeToDockerHub(r, mergeNameAndTagOrDigest(reference, query.Tag))
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, fmt.Errorf("normalizing image: %w", err))
			return
		}
		reference = possiblyNormalizedName
	}

	platformSpecs := strings.Split(query.Platform, "/")
	opts := entities.ImageImportOptions{
		Source:    source,
		Changes:   query.Changes,
		Message:   query.Message,
		Reference: reference,
		OS:        platformSpecs[0],
	}
	if len(platformSpecs) > 1 {
		opts.Architecture = platformSpecs[1]
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}
	report, err := imageEngine.Import(r.Context(), opts)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("unable to import tarball: %w", err))
		return
	}
	// Success
	utils.WriteResponse(w, http.StatusOK, struct {
		Status         string            `json:"status"`
		Progress       string            `json:"progress"`
		ProgressDetail map[string]string `json:"progressDetail"`
		Id             string            `json:"id"` //nolint:revive,stylecheck
	}{
		Status:         report.Id,
		ProgressDetail: map[string]string{},
		Id:             report.Id,
	})
}

func CreateImageFromImage(w http.ResponseWriter, r *http.Request) {
	// 200 no error
	// 404 repo does not exist or no read access
	// 500 internal
	decoder := utils.GetDecoder(r)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		FromImage  string `schema:"fromImage"`
		Tag        string `schema:"tag"`
		Platform   string `schema:"platform"`
		Retry      uint   `schema:"retry"`
		RetryDelay string `schema:"retryDelay"`
	}{
		// This is where you can override the golang default value for one of fields
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	possiblyNormalizedName, err := utils.NormalizeToDockerHub(r, mergeNameAndTagOrDigest(query.FromImage, query.Tag))
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("normalizing image: %w", err))
		return
	}

	authConf, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
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

	if _, found := r.URL.Query()["retry"]; found {
		pullOptions.MaxRetries = &query.Retry
	}

	if _, found := r.URL.Query()["retryDelay"]; found {
		duration, err := time.ParseDuration(query.RetryDelay)
		if err != nil {
			utils.Error(w, http.StatusBadRequest, err)
			return
		}
		pullOptions.RetryDelay = &duration
	}

	// Handle the platform.
	platformSpecs := strings.Split(query.Platform, "/")
	pullOptions.OS = platformSpecs[0] // may be empty
	if len(platformSpecs) > 1 {
		pullOptions.Architecture = platformSpecs[1]
		if len(platformSpecs) > 2 {
			pullOptions.Variant = platformSpecs[2]
		}
	}

	utils.CompatPull(r.Context(), w, runtime, possiblyNormalizedName, config.PullPolicyAlways, pullOptions)
}

func GetImage(w http.ResponseWriter, r *http.Request) {
	// 200 no error
	// 404 no such
	// 500 internal
	name := utils.GetName(r)
	possiblyNormalizedName, err := utils.NormalizeToDockerHub(r, name)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("normalizing image: %w", err))
		return
	}

	newImage, err := utils.GetImage(r, possiblyNormalizedName)
	if err != nil {
		// Here we need to fiddle with the error message because docker-py is looking for "No
		// such image" to determine on how to raise the correct exception.
		errMsg := strings.ReplaceAll(err.Error(), "image not known", "No such image")
		utils.Error(w, http.StatusNotFound, fmt.Errorf("failed to find image %s: %s", name, errMsg))
		return
	}
	inspect, err := imageDataToImageInspect(r.Context(), newImage)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to convert ImageData to ImageInspect '%s': %w", name, err))
		return
	}
	utils.WriteResponse(w, http.StatusOK, inspect)
}

func imageDataToImageInspect(ctx context.Context, l *libimage.Image) (*handlers.ImageInspect, error) {
	options := &libimage.InspectOptions{WithParent: true, WithSize: true}
	info, err := l.Inspect(ctx, options)
	if err != nil {
		return nil, err
	}
	ports, err := portsToPortSet(info.Config.ExposedPorts)
	if err != nil {
		return nil, err
	}

	// TODO: many fields in Config still need wiring
	config := dockerContainer.Config{
		User:         info.User,
		ExposedPorts: ports,
		Env:          info.Config.Env,
		Cmd:          info.Config.Cmd,
		Volumes:      info.Config.Volumes,
		WorkingDir:   info.Config.WorkingDir,
		Entrypoint:   info.Config.Entrypoint,
		Labels:       info.Labels,
		StopSignal:   info.Config.StopSignal,
	}

	rootfs := docker.RootFS{}
	if info.RootFS != nil {
		rootfs.Type = info.RootFS.Type
		rootfs.Layers = make([]string, 0, len(info.RootFS.Layers))
		for _, layer := range info.RootFS.Layers {
			rootfs.Layers = append(rootfs.Layers, string(layer))
		}
	}

	graphDriver := docker.GraphDriverData{
		Name: info.GraphDriver.Name,
		Data: info.GraphDriver.Data,
	}
	// Add in basic ContainerConfig to satisfy docker-compose
	cc := new(dockerContainer.Config)
	cc.Hostname = info.ID[0:11] // short ID is the hostname
	cc.Volumes = info.Config.Volumes

	dockerImageInspect := docker.ImageInspect{
		Architecture:    info.Architecture,
		Author:          info.Author,
		Comment:         info.Comment,
		Config:          &config,
		ContainerConfig: cc,
		Created:         l.Created().Format(time.RFC3339Nano),
		DockerVersion:   info.Version,
		GraphDriver:     graphDriver,
		ID:              "sha256:" + l.ID(),
		Metadata:        dockerImage.Metadata{},
		Os:              info.Os,
		OsVersion:       info.Version,
		Parent:          info.Parent,
		RepoDigests:     info.RepoDigests,
		RepoTags:        info.RepoTags,
		RootFS:          rootfs,
		Size:            info.Size,
		Variant:         "",
		VirtualSize:     info.VirtualSize,
	}
	return &handlers.ImageInspect{ImageInspect: dockerImageInspect}, nil
}

// portsToPortSet converts libpod's exposed ports to docker's structs
func portsToPortSet(input map[string]struct{}) (nat.PortSet, error) {
	ports := make(nat.PortSet)
	for k := range input {
		proto, port := nat.SplitProtoPort(k)
		switch proto {
		// See the OCI image spec for details:
		// https://github.com/opencontainers/image-spec/blob/e562b04403929d582d449ae5386ff79dd7961a11/config.md#properties
		case "tcp", "":
			p, err := nat.NewPort("tcp", port)
			if err != nil {
				return nil, fmt.Errorf("unable to create tcp port from %s: %w", k, err)
			}
			ports[p] = struct{}{}
		case "udp":
			p, err := nat.NewPort("udp", port)
			if err != nil {
				return nil, fmt.Errorf("unable to create tcp port from %s: %w", k, err)
			}
			ports[p] = struct{}{}
		default:
			return nil, fmt.Errorf("invalid port proto %q in %q", proto, k)
		}
	}
	return ports, nil
}

func GetImages(w http.ResponseWriter, r *http.Request) {
	decoder := utils.GetDecoder(r)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	query := struct {
		All     bool
		Digests bool
		Filter  string // Docker 1.24 compatibility
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest,
			fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	if _, found := r.URL.Query()["digests"]; found && query.Digests {
		utils.UnSupportedParameter("digests")
		return
	}

	var filterList []string
	var err error
	if utils.IsLibpodRequest(r) {
		// Podman clients split the filter map as `"{"label":["version","1.0"]}`
		filterList, err = filters.FiltersFromRequest(r)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, err)
			return
		}
	} else {
		// Docker clients split the filter map as `"{"label":["version=1.0"]}`
		filterList, err = util.FiltersFromRequest(r)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, err)
			return
		}
		if len(query.Filter) > 0 { // Docker 1.24 compatibility
			filterList = append(filterList, "reference="+query.Filter)
		}
		filterList = append(filterList, "manifest=false")
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}

	listOptions := entities.ImageListOptions{All: query.All, Filter: filterList, ExtendedAttributes: utils.IsLibpodRequest(r)}
	summaries, err := imageEngine.List(r.Context(), listOptions)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}

	if !utils.IsLibpodRequest(r) {
		// docker adds sha256: in front of the ID
		for _, s := range summaries {
			s.ID = "sha256:" + s.ID
		}
	}
	utils.WriteResponse(w, http.StatusOK, summaries)
}

func LoadImages(w http.ResponseWriter, r *http.Request) {
	decoder := utils.GetDecoder(r)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		Changes map[string]string `json:"changes"` // Ignored
		Message string            `json:"message"` // Ignored
		Quiet   bool              `json:"quiet"`   // Ignored
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	// First write the body to a temporary file that we can later attempt
	// to load.
	f, err := os.CreateTemp("", "api_load.tar")
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to create tempfile: %w", err))
		return
	}
	defer func() {
		err := os.Remove(f.Name())
		if err != nil {
			logrus.Errorf("Failed to remove temporary file: %v.", err)
		}
	}()
	if err := SaveFromBody(f, r); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to write temporary file: %w", err))
		return
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}

	loadOptions := entities.ImageLoadOptions{Input: f.Name()}
	loadReport, err := imageEngine.Load(r.Context(), loadOptions)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to load image: %w", err))
		return
	}

	if len(loadReport.Names) < 1 {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("one or more images are required"))
		return
	}

	utils.WriteResponse(w, http.StatusOK, struct {
		Stream string `json:"stream"`
	}{
		Stream: fmt.Sprintf("Loaded image: %s", strings.Join(loadReport.Names, ",")),
	})
}

func ExportImages(w http.ResponseWriter, r *http.Request) {
	// 200 OK
	// 500 Error
	decoder := utils.GetDecoder(r)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		Names []string `schema:"names"`
	}{
		// This is where you can override the golang default value for one of fields
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	if len(query.Names) == 0 {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("no images to download"))
		return
	}

	images := make([]string, len(query.Names))
	for i, img := range query.Names {
		possiblyNormalizedName, err := utils.NormalizeToDockerHub(r, img)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, fmt.Errorf("normalizing image: %w", err))
			return
		}
		images[i] = possiblyNormalizedName
	}

	tmpfile, err := os.CreateTemp("", "api.tar")
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("unable to create tempfile: %w", err))
		return
	}
	defer os.Remove(tmpfile.Name())
	if err := tmpfile.Close(); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("unable to close tempfile: %w", err))
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
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to read the exported tarfile: %w", err))
		return
	}
	defer rdr.Close()
	utils.WriteResponse(w, http.StatusOK, rdr)
}
