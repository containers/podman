package varlinkapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/image/docker"
	"github.com/containers/image/types"
	"github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	sysreg "github.com/containers/libpod/pkg/registries"
	"github.com/containers/libpod/pkg/util"
	"github.com/docker/go-units"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/projectatomic/buildah"
	"github.com/projectatomic/buildah/imagebuildah"
)

// ListImages lists all the images in the store
// It requires no inputs.
func (i *LibpodAPI) ListImages(call iopodman.VarlinkCall) error {
	images, err := i.Runtime.ImageRuntime().GetImages()
	if err != nil {
		return call.ReplyErrorOccurred(fmt.Sprintf("unable to get list of images %q", err))
	}
	var imageList []iopodman.ImageInList
	for _, image := range images {
		labels, _ := image.Labels(getContext())
		containers, _ := image.Containers()
		size, _ := image.Size(getContext())

		i := iopodman.ImageInList{
			Id:          image.ID(),
			ParentId:    image.Parent,
			RepoTags:    image.Names(),
			RepoDigests: image.RepoDigests(),
			Created:     image.Created().String(),
			Size:        int64(*size),
			VirtualSize: image.VirtualSize,
			Containers:  int64(len(containers)),
			Labels:      labels,
		}
		imageList = append(imageList, i)
	}
	return call.ReplyListImages(imageList)
}

// GetImage returns a single image in the form of a ImageInList
func (i *LibpodAPI) GetImage(call iopodman.VarlinkCall, name string) error {
	newImage, err := i.Runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	labels, err := newImage.Labels(getContext())
	if err != nil {
		return err
	}
	containers, err := newImage.Containers()
	if err != nil {
		return err
	}
	size, err := newImage.Size(getContext())
	if err != nil {
		return err
	}

	il := iopodman.ImageInList{
		Id:          newImage.ID(),
		ParentId:    newImage.Parent,
		RepoTags:    newImage.Names(),
		RepoDigests: newImage.RepoDigests(),
		Created:     newImage.Created().String(),
		Size:        int64(*size),
		VirtualSize: newImage.VirtualSize,
		Containers:  int64(len(containers)),
		Labels:      labels,
	}
	return call.ReplyGetImage(il)
}

// BuildImage ...
func (i *LibpodAPI) BuildImage(call iopodman.VarlinkCall, config iopodman.BuildInfo) error {
	var (
		memoryLimit int64
		memorySwap  int64
		namespace   []buildah.NamespaceOption
		err         error
	)

	systemContext := types.SystemContext{}
	dockerfiles := config.Dockerfile
	contextDir := ""

	for i := range dockerfiles {
		if strings.HasPrefix(dockerfiles[i], "http://") ||
			strings.HasPrefix(dockerfiles[i], "https://") ||
			strings.HasPrefix(dockerfiles[i], "git://") ||
			strings.HasPrefix(dockerfiles[i], "github.com/") {
			continue
		}
		absFile, err := filepath.Abs(dockerfiles[i])
		if err != nil {
			return errors.Wrapf(err, "error determining path to file %q", dockerfiles[i])
		}
		contextDir = filepath.Dir(absFile)
		dockerfiles[i], err = filepath.Rel(contextDir, absFile)
		if err != nil {
			return errors.Wrapf(err, "error determining path to file %q", dockerfiles[i])
		}
		break
	}

	pullPolicy := imagebuildah.PullNever
	if config.Pull {
		pullPolicy = imagebuildah.PullIfMissing
	}

	if config.Pull_always {
		pullPolicy = imagebuildah.PullAlways
	}

	format := "oci"
	if config.Image_format != "" {
		format = config.Image_format
	}

	if strings.HasPrefix(format, "oci") {
		format = imagebuildah.OCIv1ImageFormat
	} else if strings.HasPrefix(format, "docker") {
		format = imagebuildah.Dockerv2ImageFormat
	} else {
		return call.ReplyErrorOccurred(fmt.Sprintf("unrecognized image type %q", format))
	}

	if config.Memory != "" {
		memoryLimit, err = units.RAMInBytes(config.Memory)
		if err != nil {
			return call.ReplyErrorOccurred(err.Error())
		}
	}

	if config.Memory_swap != "" {
		memorySwap, err = units.RAMInBytes(config.Memory_swap)
		if err != nil {
			return call.ReplyErrorOccurred(err.Error())
		}
	}

	output := bytes.NewBuffer([]byte{})
	commonOpts := &buildah.CommonBuildOptions{
		AddHost:      config.Add_hosts,
		CgroupParent: config.Cgroup_parent,
		CPUPeriod:    uint64(config.Cpu_period),
		CPUQuota:     config.Cpu_quota,
		CPUSetCPUs:   config.Cpuset_cpus,
		CPUSetMems:   config.Cpuset_mems,
		Memory:       memoryLimit,
		MemorySwap:   memorySwap,
		ShmSize:      config.Shm_size,
		Ulimit:       config.Ulimit,
		Volumes:      config.Volume,
	}

	hostNetwork := buildah.NamespaceOption{
		Name: specs.NetworkNamespace,
		Host: true,
	}

	namespace = append(namespace, hostNetwork)

	options := imagebuildah.BuildOptions{
		ContextDirectory: contextDir,
		PullPolicy:       pullPolicy,
		Compression:      imagebuildah.Gzip,
		Quiet:            false,
		//SignaturePolicyPath:
		Args: config.Build_args,
		//Output:
		AdditionalTags: config.Tags,
		//Runtime: runtime.
		//RuntimeArgs: ,
		OutputFormat:     format,
		SystemContext:    &systemContext,
		CommonBuildOpts:  commonOpts,
		Squash:           config.Squash,
		Labels:           config.Label,
		Annotations:      config.Annotations,
		ReportWriter:     output,
		NamespaceOptions: namespace,
	}

	if call.WantsMore() {
		call.Continues = true
	}

	c := build(i.Runtime, options, config.Dockerfile)
	var log []string
	done := false
	for {
		line, err := output.ReadString('\n')
		if err == nil {
			log = append(log, line)
			continue
		} else if err == io.EOF {
			select {
			case err := <-c:
				if err != nil {
					return call.ReplyErrorOccurred(err.Error())
				}
				done = true
			default:
				if !call.WantsMore() {
					time.Sleep(1 * time.Second)
					break
				}
				br := iopodman.BuildResponse{
					Logs: log,
				}
				call.ReplyBuildImage(br)
				log = []string{}
			}
		} else {
			return call.ReplyErrorOccurred(err.Error())
		}
		if done {
			break
		}
	}
	call.Continues = false
	newImage, err := i.Runtime.ImageRuntime().NewFromLocal(config.Tags[0])
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	br := iopodman.BuildResponse{
		Logs: log,
		Id:   newImage.ID(),
	}
	return call.ReplyBuildImage(br)
}

func build(runtime *libpod.Runtime, options imagebuildah.BuildOptions, dockerfiles []string) chan error {
	c := make(chan error)
	go func() {
		err := runtime.Build(getContext(), options, dockerfiles...)
		c <- err
		close(c)
	}()

	return c
}

// CreateImage ...
// TODO With Pull being added, should we skip Create?
func (i *LibpodAPI) CreateImage(call iopodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("CreateImage")
}

// InspectImage returns an image's inspect information as a string that can be serialized.
// Requires an image ID or name
func (i *LibpodAPI) InspectImage(call iopodman.VarlinkCall, name string) error {
	newImage, err := i.Runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		return call.ReplyImageNotFound(name)
	}
	inspectInfo, err := newImage.Inspect(getContext())
	b, err := json.Marshal(inspectInfo)
	if err != nil {
		return call.ReplyErrorOccurred(fmt.Sprintf("unable to serialize"))
	}
	return call.ReplyInspectImage(string(b))
}

// HistoryImage returns the history of the image's layers
// Requires an image or name
func (i *LibpodAPI) HistoryImage(call iopodman.VarlinkCall, name string) error {
	newImage, err := i.Runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		return call.ReplyImageNotFound(name)
	}
	history, err := newImage.History(getContext())
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	var histories []iopodman.ImageHistory
	for _, hist := range history {
		imageHistory := iopodman.ImageHistory{
			Id:        hist.ID,
			Created:   hist.Created.String(),
			CreatedBy: hist.CreatedBy,
			Tags:      newImage.Names(),
			Size:      hist.Size,
			Comment:   hist.Comment,
		}
		histories = append(histories, imageHistory)
	}
	return call.ReplyHistoryImage(histories)
}

// PushImage pushes an local image to registry
// TODO We need to add options for signing, credentials, tls, and multi-tag
func (i *LibpodAPI) PushImage(call iopodman.VarlinkCall, name, tag string, tlsVerify bool) error {
	newImage, err := i.Runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		return call.ReplyImageNotFound(err.Error())
	}
	destname := name
	if tag != "" {
		destname = tag
	}

	dockerRegistryOptions := image.DockerRegistryOptions{
		DockerInsecureSkipTLSVerify: !tlsVerify,
	}

	so := image.SigningOptions{}

	if err := newImage.PushImageToHeuristicDestination(getContext(), destname, "", "", "", nil, false, so, &dockerRegistryOptions, false, nil); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyPushImage(newImage.ID())
}

// TagImage accepts an image name and tag as strings and tags an image in the local store.
func (i *LibpodAPI) TagImage(call iopodman.VarlinkCall, name, tag string) error {
	newImage, err := i.Runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		return call.ReplyImageNotFound(name)
	}
	if err := newImage.TagImage(tag); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyTagImage(newImage.ID())
}

// RemoveImage accepts a image name or ID as a string and force bool to determine if it should
// remove the image even if being used by stopped containers
func (i *LibpodAPI) RemoveImage(call iopodman.VarlinkCall, name string, force bool) error {
	ctx := getContext()
	newImage, err := i.Runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		return call.ReplyImageNotFound(name)
	}
	_, err = i.Runtime.RemoveImage(ctx, newImage, force)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyRemoveImage(newImage.ID())
}

// SearchImage searches all registries configured in /etc/containers/registries.conf for an image
// Requires an image name and a search limit as int
func (i *LibpodAPI) SearchImage(call iopodman.VarlinkCall, name string, limit int64) error {
	sc := image.GetSystemContext("", "", false)
	registries, err := sysreg.GetRegistries()
	if err != nil {
		return call.ReplyErrorOccurred(fmt.Sprintf("unable to get system registries: %q", err))
	}
	var imageResults []iopodman.ImageSearch
	for _, reg := range registries {
		results, err := docker.SearchRegistry(getContext(), sc, reg, name, int(limit))
		if err != nil {
			// If we are searching multiple registries, don't make something like an
			// auth error fatal. Unfortunately we cannot differentiate between auth
			// errors and other possibles errors
			if len(registries) > 1 {
				continue
			}
			return call.ReplyErrorOccurred(err.Error())
		}
		for _, result := range results {
			i := iopodman.ImageSearch{
				Description:  result.Description,
				Is_official:  result.IsOfficial,
				Is_automated: result.IsAutomated,
				Name:         result.Name,
				Star_count:   int64(result.StarCount),
			}
			imageResults = append(imageResults, i)
		}
	}
	return call.ReplySearchImage(imageResults)
}

// DeleteUnusedImages deletes any images that do not have containers associated with it.
// TODO Filters are not implemented
func (i *LibpodAPI) DeleteUnusedImages(call iopodman.VarlinkCall) error {
	images, err := i.Runtime.ImageRuntime().GetImages()
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	var deletedImages []string
	for _, img := range images {
		containers, err := img.Containers()
		if err != nil {
			return call.ReplyErrorOccurred(err.Error())
		}
		if len(containers) == 0 {
			if err := img.Remove(false); err != nil {
				return call.ReplyErrorOccurred(err.Error())
			}
			deletedImages = append(deletedImages, img.ID())
		}
	}
	return call.ReplyDeleteUnusedImages(deletedImages)
}

// Commit ...
func (i *LibpodAPI) Commit(call iopodman.VarlinkCall, name, imageName string, changes []string, author, message string, pause bool) error {
	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name)
	}
	sc := image.GetSystemContext(i.Runtime.GetConfig().SignaturePolicyPath, "", false)
	coptions := buildah.CommitOptions{
		SignaturePolicyPath:   i.Runtime.GetConfig().SignaturePolicyPath,
		ReportWriter:          nil,
		SystemContext:         sc,
		PreferredManifestType: buildah.OCIv1ImageManifest,
	}
	options := libpod.ContainerCommitOptions{
		CommitOptions: coptions,
		Pause:         pause,
		Message:       message,
		Changes:       changes,
		Author:        author,
	}

	newImage, err := ctr.Commit(getContext(), imageName, options)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyCommit(newImage.ID())
}

// ImportImage imports an image from a tarball to the image store
func (i *LibpodAPI) ImportImage(call iopodman.VarlinkCall, source, reference, message string, changes []string) error {
	configChanges, err := util.GetImageConfig(changes)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	history := []v1.History{
		{Comment: message},
	}
	config := v1.Image{
		Config:  configChanges,
		History: history,
	}
	newImage, err := i.Runtime.ImageRuntime().Import(getContext(), source, reference, nil, image.SigningOptions{}, config)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyImportImage(newImage.ID())
}

// ExportImage exports an image to the provided destination
// destination must have the transport type!!
func (i *LibpodAPI) ExportImage(call iopodman.VarlinkCall, name, destination string, compress bool, tags []string) error {
	newImage, err := i.Runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		return call.ReplyImageNotFound(name)
	}

	additionalTags, err := image.GetAdditionalTags(tags)
	if err != nil {
		return err
	}

	if err := newImage.PushImageToHeuristicDestination(getContext(), destination, "", "", "", nil, compress, image.SigningOptions{}, &image.DockerRegistryOptions{}, false, additionalTags); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyExportImage(newImage.ID())
}

// PullImage pulls an image from a registry to the image store.
// TODO This implementation is incomplete
func (i *LibpodAPI) PullImage(call iopodman.VarlinkCall, name string) error {
	newImage, err := i.Runtime.ImageRuntime().New(getContext(), name, "", "", nil, &image.DockerRegistryOptions{}, image.SigningOptions{}, true, false)
	if err != nil {
		return call.ReplyErrorOccurred(fmt.Sprintf("unable to pull %s: %s", name, err.Error()))
	}
	return call.ReplyPullImage(newImage.ID())
}
