package varlinkapi

import (
	"encoding/json"
	"fmt"

	"github.com/containers/image/docker"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/projectatomic/libpod/cmd/podman/libpodruntime"
	ioprojectatomicpodman "github.com/projectatomic/libpod/cmd/podman/varlink"
	"github.com/projectatomic/libpod/libpod/image"
	sysreg "github.com/projectatomic/libpod/pkg/registries"
	"github.com/projectatomic/libpod/pkg/util"
)

// ListImages lists all the images in the store
// It requires no inputs.
func (i *LibpodAPI) ListImages(call ioprojectatomicpodman.VarlinkCall) error {
	runtime, err := libpodruntime.GetRuntime(i.Cli)
	if err != nil {
		return call.ReplyRuntimeError(err.Error())
	}
	images, err := runtime.ImageRuntime().GetImages()
	if err != nil {
		return call.ReplyErrorOccurred(fmt.Sprintf("unable to get list of images %q", err))
	}
	var imageList []ioprojectatomicpodman.ImageInList
	for _, image := range images {
		//size, _:= image.Size(getContext())
		labels, _ := image.Labels(getContext())
		containers, _ := image.Containers()

		i := ioprojectatomicpodman.ImageInList{
			Id:          image.ID(),
			ParentId:    image.Parent,
			RepoTags:    image.Names(),
			RepoDigests: image.RepoDigests(),
			Created:     image.Created().String(),
			//Size: size,
			VirtualSize: image.VirtualSize,
			Containers:  int64(len(containers)),
			Labels:      labels,
		}
		imageList = append(imageList, i)
	}
	return call.ReplyListImages(imageList)
}

// BuildImage ...
// TODO Waiting for buildah to be vendored into libpod to do this only one
func (i *LibpodAPI) BuildImage(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("BuildImage")
}

// CreateImage ...
// TODO With Pull being added, should we skip Create?
func (i *LibpodAPI) CreateImage(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("CreateImage")
}

// InspectImage returns an image's inspect information as a string that can be serialized.
// Requires an image ID or name
func (i *LibpodAPI) InspectImage(call ioprojectatomicpodman.VarlinkCall, name string) error {
	runtime, err := libpodruntime.GetRuntime(i.Cli)
	if err != nil {
		return call.ReplyRuntimeError(err.Error())
	}
	newImage, err := runtime.ImageRuntime().NewFromLocal(name)
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
func (i *LibpodAPI) HistoryImage(call ioprojectatomicpodman.VarlinkCall, name string) error {
	runtime, err := libpodruntime.GetRuntime(i.Cli)
	if err != nil {
		return call.ReplyRuntimeError(err.Error())
	}
	newImage, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		return call.ReplyImageNotFound(name)
	}
	history, layerInfos, err := newImage.History(getContext())
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	var (
		histories []ioprojectatomicpodman.ImageHistory
		count     = 1
	)
	for i := len(history) - 1; i >= 0; i-- {
		var size int64
		if !history[i].EmptyLayer {
			size = layerInfos[len(layerInfos)-count].Size
			count++
		}
		imageHistory := ioprojectatomicpodman.ImageHistory{
			Id:        newImage.ID(),
			Created:   history[i].Created.String(),
			CreatedBy: history[i].CreatedBy,
			Tags:      newImage.Names(),
			Size:      size,
			Comment:   history[i].Comment,
		}
		histories = append(histories, imageHistory)
	}
	return call.ReplyHistoryImage(histories)
}

// PushImage pushes an local image to registry
// TODO We need to add options for signing, credentials, and tls
func (i *LibpodAPI) PushImage(call ioprojectatomicpodman.VarlinkCall, name, tag string, tlsVerify bool) error {
	runtime, err := libpodruntime.GetRuntime(i.Cli)
	if err != nil {
		return call.ReplyRuntimeError(err.Error())
	}
	newImage, err := runtime.ImageRuntime().NewFromLocal(name)
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

	if err := newImage.PushImage(getContext(), destname, "", "", "", nil, false, so, &dockerRegistryOptions); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyPushImage(newImage.ID())
}

// TagImage accepts an image name and tag as strings and tags an image in the local store.
func (i *LibpodAPI) TagImage(call ioprojectatomicpodman.VarlinkCall, name, tag string) error {
	runtime, err := libpodruntime.GetRuntime(i.Cli)
	if err != nil {
		return call.ReplyRuntimeError(err.Error())
	}
	newImage, err := runtime.ImageRuntime().NewFromLocal(name)
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
func (i *LibpodAPI) RemoveImage(call ioprojectatomicpodman.VarlinkCall, name string, force bool) error {
	runtime, err := libpodruntime.GetRuntime(i.Cli)
	if err != nil {
		return call.ReplyRuntimeError(err.Error())
	}
	newImage, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		return call.ReplyImageNotFound(name)
	}
	if err := newImage.Remove(force); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyRemoveImage(newImage.ID())
}

// SearchImage searches all registries configured in /etc/containers/registries.conf for an image
// Requires an image name and a search limit as int
func (i *LibpodAPI) SearchImage(call ioprojectatomicpodman.VarlinkCall, name string, limit int64) error {
	sc := image.GetSystemContext("", "", false)
	registries, err := sysreg.GetRegistries()
	if err != nil {
		return call.ReplyErrorOccurred(fmt.Sprintf("unable to get system registries: %q", err))
	}
	var imageResults []ioprojectatomicpodman.ImageSearch
	for _, reg := range registries {
		results, err := docker.SearchRegistry(getContext(), sc, reg, name, int(limit))
		if err != nil {
			return call.ReplyErrorOccurred(err.Error())
		}
		for _, result := range results {
			i := ioprojectatomicpodman.ImageSearch{
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
func (i *LibpodAPI) DeleteUnusedImages(call ioprojectatomicpodman.VarlinkCall) error {
	runtime, err := libpodruntime.GetRuntime(i.Cli)
	if err != nil {
		return call.ReplyRuntimeError(err.Error())
	}
	images, err := runtime.ImageRuntime().GetImages()
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

// CreateFromContainer ...
// TODO This must wait until buildah is properly vendored into libpod
func (i *LibpodAPI) CreateFromContainer(call ioprojectatomicpodman.VarlinkCall) error {
	return call.ReplyMethodNotImplemented("CreateFromContainer")
}

// ImportImage imports an image from a tarball to the image store
func (i *LibpodAPI) ImportImage(call ioprojectatomicpodman.VarlinkCall, source, reference, message string, changes []string) error {
	runtime, err := libpodruntime.GetRuntime(i.Cli)
	if err != nil {
		return call.ReplyRuntimeError(err.Error())
	}
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
	newImage, err := runtime.ImageRuntime().Import(getContext(), source, reference, nil, image.SigningOptions{}, config)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyImportImage(newImage.ID())
}

// ExportImage exports an image to the provided destination
// destination must have the transport type!!
func (i *LibpodAPI) ExportImage(call ioprojectatomicpodman.VarlinkCall, name, destination string, compress bool) error {
	runtime, err := libpodruntime.GetRuntime(i.Cli)
	if err != nil {
		return call.ReplyRuntimeError(err.Error())
	}
	newImage, err := runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		return call.ReplyImageNotFound(name)
	}
	if err := newImage.PushImage(getContext(), destination, "", "", "", nil, compress, image.SigningOptions{}, &image.DockerRegistryOptions{}); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyExportImage(newImage.ID())
}

// PullImage pulls an image from a registry to the image store.
// TODO This implementation is incomplete
func (i *LibpodAPI) PullImage(call ioprojectatomicpodman.VarlinkCall, name string) error {
	runtime, err := libpodruntime.GetRuntime(i.Cli)
	if err != nil {
		return call.ReplyRuntimeError(err.Error())
	}
	newImage, err := runtime.ImageRuntime().New(getContext(), name, "", "", nil, &image.DockerRegistryOptions{}, image.SigningOptions{}, true, false)
	if err != nil {
		return call.ReplyErrorOccurred(fmt.Sprintf("unable to pull %s", name))
	}
	return call.ReplyPullImage(newImage.ID())
}
