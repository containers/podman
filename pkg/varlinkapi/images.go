// +build varlink

package varlinkapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/buildah"
	"github.com/containers/buildah/imagebuildah"
	dockerarchive "github.com/containers/image/v5/docker/archive"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/libpod/image"
	"github.com/containers/podman/v2/pkg/channel"
	"github.com/containers/podman/v2/pkg/util"
	iopodman "github.com/containers/podman/v2/pkg/varlink"
	"github.com/containers/podman/v2/utils"
	"github.com/containers/storage/pkg/archive"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ListImagesWithFilters returns a list of images that have been filtered
func (i *VarlinkAPI) ListImagesWithFilters(call iopodman.VarlinkCall, filters []string) error {
	images, err := i.Runtime.ImageRuntime().GetImagesWithFilters(filters)
	if err != nil {
		return call.ReplyErrorOccurred(fmt.Sprintf("unable to get list of images %q", err))
	}
	imageList, err := imagesToImageList(images)
	if err != nil {
		return call.ReplyErrorOccurred("unable to parse response " + err.Error())
	}
	return call.ReplyListImagesWithFilters(imageList)
}

// imagesToImageList converts a slice of Images to an imagelist for varlink responses
func imagesToImageList(images []*image.Image) ([]iopodman.Image, error) {
	var imageList []iopodman.Image
	for _, img := range images {
		labels, _ := img.Labels(getContext())
		containers, _ := img.Containers()
		repoDigests, err := img.RepoDigests()
		if err != nil {
			return nil, err
		}

		size, _ := img.Size(getContext())
		isParent, err := img.IsParent(context.TODO())
		if err != nil {
			return nil, err
		}

		i := iopodman.Image{
			Id:          img.ID(),
			Digest:      string(img.Digest()),
			ParentId:    img.Parent,
			RepoTags:    img.Names(),
			RepoDigests: repoDigests,
			Created:     img.Created().Format(time.RFC3339),
			Size:        int64(*size),
			VirtualSize: img.VirtualSize,
			Containers:  int64(len(containers)),
			Labels:      labels,
			IsParent:    isParent,
			ReadOnly:    img.IsReadOnly(),
			History:     img.NamesHistory(),
		}
		imageList = append(imageList, i)
	}
	return imageList, nil
}

// ListImages lists all the images in the store
// It requires no inputs.
func (i *VarlinkAPI) ListImages(call iopodman.VarlinkCall) error {
	images, err := i.Runtime.ImageRuntime().GetImages()
	if err != nil {
		return call.ReplyErrorOccurred("unable to get list of images " + err.Error())
	}
	imageList, err := imagesToImageList(images)
	if err != nil {
		return call.ReplyErrorOccurred("unable to parse response " + err.Error())
	}
	return call.ReplyListImages(imageList)
}

// GetImage returns a single image in the form of a Image
func (i *VarlinkAPI) GetImage(call iopodman.VarlinkCall, id string) error {
	newImage, err := i.Runtime.ImageRuntime().NewFromLocal(id)
	if err != nil {
		return call.ReplyImageNotFound(id, err.Error())
	}
	labels, err := newImage.Labels(getContext())
	if err != nil {
		return err
	}
	containers, err := newImage.Containers()
	if err != nil {
		return err
	}
	repoDigests, err := newImage.RepoDigests()
	if err != nil {
		return err
	}
	size, err := newImage.Size(getContext())
	if err != nil {
		return err
	}

	il := iopodman.Image{
		Id:          newImage.ID(),
		ParentId:    newImage.Parent,
		RepoTags:    newImage.Names(),
		RepoDigests: repoDigests,
		Created:     newImage.Created().Format(time.RFC3339),
		Size:        int64(*size),
		VirtualSize: newImage.VirtualSize,
		Containers:  int64(len(containers)),
		Labels:      labels,
		TopLayer:    newImage.TopLayer(),
		ReadOnly:    newImage.IsReadOnly(),
		History:     newImage.NamesHistory(),
	}
	return call.ReplyGetImage(il)
}

// BuildImage ...
func (i *VarlinkAPI) BuildImage(call iopodman.VarlinkCall, config iopodman.BuildInfo) error {
	var (
		namespace []buildah.NamespaceOption
		imageID   string
		err       error
	)

	contextDir := config.ContextDir

	newContextDir, err := ioutil.TempDir("", "buildTarball")
	if err != nil {
		call.ReplyErrorOccurred("unable to create tempdir")
	}
	logrus.Debugf("created new context dir at %s", newContextDir)

	reader, err := os.Open(contextDir)
	if err != nil {
		logrus.Errorf("failed to open the context dir tar file")
		return call.ReplyErrorOccurred(fmt.Sprintf("unable to open context dir tar file %s", contextDir))
	}
	defer reader.Close()
	if err := archive.Untar(reader, newContextDir, &archive.TarOptions{}); err != nil {
		logrus.Errorf("fail to untar the context dir tarball (%s) to the context dir (%s)", contextDir, newContextDir)
		return call.ReplyErrorOccurred(fmt.Sprintf("unable to untar context dir %s", contextDir))
	}
	logrus.Debugf("untar of %s successful", contextDir)
	defer func() {
		if err := os.Remove(contextDir); err != nil {
			logrus.Error(err)
		}
		if err := os.RemoveAll(newContextDir); err != nil {
			logrus.Errorf("unable to delete directory '%s': %q", newContextDir, err)
		}
	}()

	systemContext := types.SystemContext{}
	// All output (stdout, stderr) is captured in output as well
	var output bytes.Buffer

	commonOpts := &buildah.CommonBuildOptions{
		AddHost:      config.BuildOptions.AddHosts,
		CgroupParent: config.BuildOptions.CgroupParent,
		CPUPeriod:    uint64(config.BuildOptions.CpuPeriod),
		CPUQuota:     config.BuildOptions.CpuQuota,
		CPUSetCPUs:   config.BuildOptions.CpusetCpus,
		CPUSetMems:   config.BuildOptions.CpusetMems,
		Memory:       config.BuildOptions.Memory,
		MemorySwap:   config.BuildOptions.MemorySwap,
		ShmSize:      config.BuildOptions.ShmSize,
		Ulimit:       config.BuildOptions.Ulimit,
		Volumes:      config.BuildOptions.Volume,
	}

	options := imagebuildah.BuildOptions{
		AddCapabilities:         config.AddCapabilities,
		AdditionalTags:          config.AdditionalTags,
		Annotations:             config.Annotations,
		Architecture:            config.Architecture,
		Args:                    config.BuildArgs,
		CNIConfigDir:            config.CniConfigDir,
		CNIPluginPath:           config.CniPluginDir,
		CommonBuildOpts:         commonOpts,
		Compression:             stringCompressionToArchiveType(config.Compression),
		ContextDirectory:        newContextDir,
		DefaultMountsFilePath:   config.DefaultsMountFilePath,
		Devices:                 config.Devices,
		Err:                     &output,
		ForceRmIntermediateCtrs: config.ForceRmIntermediateCtrs,
		IIDFile:                 config.Iidfile,
		Labels:                  config.Label,
		Layers:                  config.Layers,
		NamespaceOptions:        namespace,
		NoCache:                 config.Nocache,
		OS:                      config.Os,
		Out:                     &output,
		Output:                  config.Output,
		OutputFormat:            config.OutputFormat,
		PullPolicy:              stringPullPolicyToType(config.PullPolicy),
		Quiet:                   config.Quiet,
		RemoveIntermediateCtrs:  config.RemoteIntermediateCtrs,
		ReportWriter:            &output,
		RuntimeArgs:             config.RuntimeArgs,
		SignBy:                  config.SignBy,
		Squash:                  config.Squash,
		SystemContext:           &systemContext,
		Target:                  config.Target,
		TransientMounts:         config.TransientMounts,
	}

	if call.WantsMore() {
		call.Continues = true
	} else {
		return call.ReplyErrorOccurred("endpoint requires a more connection")
	}

	var newPathDockerFiles []string

	for _, d := range config.Dockerfiles {
		if strings.HasPrefix(d, "http://") ||
			strings.HasPrefix(d, "https://") ||
			strings.HasPrefix(d, "git://") ||
			strings.HasPrefix(d, "github.com/") {
			newPathDockerFiles = append(newPathDockerFiles, d)
			continue
		}
		base := filepath.Base(d)
		newPathDockerFiles = append(newPathDockerFiles, filepath.Join(newContextDir, base))
	}

	c := make(chan error)
	go func() {
		iid, _, err := i.Runtime.Build(getContext(), options, newPathDockerFiles...)
		imageID = iid
		c <- err
		close(c)
	}()

	var log []string
	done := false
	for {
		outputLine, err := output.ReadString('\n')
		if err == nil {
			log = append(log, outputLine)
			if call.WantsMore() {
				//	 we want to reply with what we have
				br := iopodman.MoreResponse{
					Logs: log,
				}
				call.ReplyBuildImage(br)
				log = []string{}
			}
			continue
		} else if err == io.EOF {
			select {
			case err := <-c:
				if err != nil {
					return call.ReplyErrorOccurred(err.Error())
				}
				done = true
			default:
				if call.WantsMore() {
					time.Sleep(1 * time.Second)
					break
				}
			}
		} else {
			return call.ReplyErrorOccurred(err.Error())
		}
		if done {
			break
		}
	}
	call.Continues = false

	br := iopodman.MoreResponse{
		Logs: log,
		Id:   imageID,
	}
	return call.ReplyBuildImage(br)
}

// InspectImage returns an image's inspect information as a string that can be serialized.
// Requires an image ID or name
func (i *VarlinkAPI) InspectImage(call iopodman.VarlinkCall, name string) error {
	newImage, err := i.Runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		return call.ReplyImageNotFound(name, err.Error())
	}
	inspectInfo, err := newImage.Inspect(getContext())
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	b, err := json.Marshal(inspectInfo)
	if err != nil {
		return call.ReplyErrorOccurred(fmt.Sprintf("unable to serialize"))
	}
	return call.ReplyInspectImage(string(b))
}

// HistoryImage returns the history of the image's layers
// Requires an image or name
func (i *VarlinkAPI) HistoryImage(call iopodman.VarlinkCall, name string) error {
	newImage, err := i.Runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		return call.ReplyImageNotFound(name, err.Error())
	}
	history, err := newImage.History(getContext())
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	var histories []iopodman.ImageHistory
	for _, hist := range history {
		imageHistory := iopodman.ImageHistory{
			Id:        hist.ID,
			Created:   hist.Created.Format(time.RFC3339),
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
func (i *VarlinkAPI) PushImage(call iopodman.VarlinkCall, name, tag string, compress bool, format string, removeSignatures bool, signBy string) error {
	var (
		manifestType string
	)
	newImage, err := i.Runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		return call.ReplyImageNotFound(name, err.Error())
	}
	destname := name
	if tag != "" {
		destname = tag
	}
	dockerRegistryOptions := image.DockerRegistryOptions{}
	if format != "" {
		switch format {
		case "oci": // nolint
			manifestType = v1.MediaTypeImageManifest
		case "v2s1":
			manifestType = manifest.DockerV2Schema1SignedMediaType
		case "v2s2", "docker":
			manifestType = manifest.DockerV2Schema2MediaType
		default:
			return call.ReplyErrorOccurred(fmt.Sprintf("unknown format %q. Choose on of the supported formats: 'oci', 'v2s1', or 'v2s2'", format))
		}
	}
	so := image.SigningOptions{
		RemoveSignatures: removeSignatures,
		SignBy:           signBy,
	}

	if call.WantsMore() {
		call.Continues = true
	}

	output := bytes.NewBuffer([]byte{})
	c := make(chan error)
	go func() {
		writer := bytes.NewBuffer([]byte{})
		err := newImage.PushImageToHeuristicDestination(getContext(), destname, manifestType, "", "", "", writer, compress, so, &dockerRegistryOptions, nil)
		if err != nil {
			c <- err
		}
		_, err = io.CopyBuffer(output, writer, nil)
		c <- err
		close(c)
	}()

	// TODO When pull output gets fixed for the remote client, we need to look into how we can turn below
	// into something re-usable.  it is in build too
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
					logrus.Errorf("reading of output during push failed for %s", newImage.ID())
					return call.ReplyErrorOccurred(err.Error())
				}
				done = true
			default:
				if !call.WantsMore() {
					break
				}
				br := iopodman.MoreResponse{
					Logs: log,
					Id:   newImage.ID(),
				}
				call.ReplyPushImage(br)
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

	br := iopodman.MoreResponse{
		Logs: log,
		Id:   newImage.ID(),
	}
	return call.ReplyPushImage(br)
}

// TagImage accepts an image name and tag as strings and tags an image in the local store.
func (i *VarlinkAPI) TagImage(call iopodman.VarlinkCall, name, tag string) error {
	newImage, err := i.Runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		return call.ReplyImageNotFound(name, err.Error())
	}
	if err := newImage.TagImage(tag); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyTagImage(newImage.ID())
}

// UntagImage accepts an image name and tag as strings and removes the tag from the local store.
func (i *VarlinkAPI) UntagImage(call iopodman.VarlinkCall, name, tag string) error {
	newImage, err := i.Runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		return call.ReplyImageNotFound(name, err.Error())
	}
	if err := newImage.UntagImage(tag); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyUntagImage(newImage.ID())
}

// RemoveImage accepts a image name or ID as a string and force bool to determine if it should
// remove the image even if being used by stopped containers
func (i *VarlinkAPI) RemoveImage(call iopodman.VarlinkCall, name string, force bool) error {
	ctx := getContext()
	newImage, err := i.Runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		return call.ReplyImageNotFound(name, err.Error())
	}
	_, err = i.Runtime.RemoveImage(ctx, newImage, force)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyRemoveImage(newImage.ID())
}

// RemoveImageWithResponse accepts an image name and force bool.  It returns details about what
// was done in removeimageresponse struct.
func (i *VarlinkAPI) RemoveImageWithResponse(call iopodman.VarlinkCall, name string, force bool) error {
	ir := iopodman.RemoveImageResponse{}
	ctx := getContext()
	newImage, err := i.Runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		return call.ReplyImageNotFound(name, err.Error())
	}
	response, err := i.Runtime.RemoveImage(ctx, newImage, force)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	ir.Untagged = append(ir.Untagged, response.Untagged...)
	ir.Deleted = response.Deleted
	return call.ReplyRemoveImageWithResponse(ir)
}

// SearchImages searches all registries configured in /etc/containers/registries.conf for an image
// Requires an image name and a search limit as int
func (i *VarlinkAPI) SearchImages(call iopodman.VarlinkCall, query string, limit *int64, filter iopodman.ImageSearchFilter) error {
	// Transform all arguments to proper types first
	argLimit := 0
	argIsOfficial := types.OptionalBoolUndefined
	argIsAutomated := types.OptionalBoolUndefined
	if limit != nil {
		argLimit = int(*limit)
	}
	if filter.Is_official != nil {
		argIsOfficial = types.NewOptionalBool(*filter.Is_official)
	}
	if filter.Is_automated != nil {
		argIsAutomated = types.NewOptionalBool(*filter.Is_automated)
	}

	// Transform a SearchFilter the backend can deal with
	sFilter := image.SearchFilter{
		IsOfficial:  argIsOfficial,
		IsAutomated: argIsAutomated,
		Stars:       int(filter.Star_count),
	}

	searchOptions := image.SearchOptions{
		Limit:  argLimit,
		Filter: sFilter,
	}
	results, err := image.SearchImages(query, searchOptions)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}

	var imageResults []iopodman.ImageSearchResult
	for _, result := range results {
		i := iopodman.ImageSearchResult{
			Registry:     result.Index,
			Description:  result.Description,
			Is_official:  result.Official == "[OK]",
			Is_automated: result.Automated == "[OK]",
			Name:         result.Name,
			Star_count:   int64(result.Stars),
		}
		imageResults = append(imageResults, i)
	}
	return call.ReplySearchImages(imageResults)
}

// DeleteUnusedImages deletes any images that do not have containers associated with it.
// TODO Filters are not implemented
func (i *VarlinkAPI) DeleteUnusedImages(call iopodman.VarlinkCall) error {
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
			if err := img.Remove(context.TODO(), false); err != nil {
				return call.ReplyErrorOccurred(err.Error())
			}
			deletedImages = append(deletedImages, img.ID())
		}
	}
	return call.ReplyDeleteUnusedImages(deletedImages)
}

// Commit ...
func (i *VarlinkAPI) Commit(call iopodman.VarlinkCall, name, imageName string, changes []string, author, message string, pause bool, manifestType string) error {
	var (
		newImage *image.Image
		log      []string
		mimeType string
	)
	output := channel.NewWriter(make(chan []byte))
	channelClose := func() {
		if err := output.Close(); err != nil {
			logrus.Errorf("failed to close channel writer: %q", err)
		}
	}
	defer channelClose()

	ctr, err := i.Runtime.LookupContainer(name)
	if err != nil {
		return call.ReplyContainerNotFound(name, err.Error())
	}
	rtc, err := i.Runtime.GetConfig()
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	sc := image.GetSystemContext(rtc.Engine.SignaturePolicyPath, "", false)
	switch manifestType {
	case "oci", "": // nolint
		mimeType = buildah.OCIv1ImageManifest
	case "docker":
		mimeType = manifest.DockerV2Schema2MediaType
	default:
		return call.ReplyErrorOccurred(fmt.Sprintf("unrecognized image format %q", manifestType))
	}
	coptions := buildah.CommitOptions{
		SignaturePolicyPath:   rtc.Engine.SignaturePolicyPath,
		ReportWriter:          output,
		SystemContext:         sc,
		PreferredManifestType: mimeType,
	}
	options := libpod.ContainerCommitOptions{
		CommitOptions: coptions,
		Pause:         pause,
		Message:       message,
		Changes:       changes,
		Author:        author,
	}

	if call.WantsMore() {
		call.Continues = true
	}

	c := make(chan error)

	go func() {
		newImage, err = ctr.Commit(getContext(), imageName, options)
		if err != nil {
			c <- err
		}
		c <- nil
		close(c)
	}()

	// reply is the func being sent to the output forwarder.  in this case it is replying
	// with a more response struct
	reply := func(br iopodman.MoreResponse) error {
		return call.ReplyCommit(br)
	}
	log, err = forwardOutput(log, c, call.WantsMore(), output, reply)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	call.Continues = false
	br := iopodman.MoreResponse{
		Logs: log,
		Id:   newImage.ID(),
	}
	return call.ReplyCommit(br)
}

// ImportImage imports an image from a tarball to the image store
func (i *VarlinkAPI) ImportImage(call iopodman.VarlinkCall, source, reference, message string, changes []string, delete bool) error {
	configChanges, err := util.GetImageConfig(changes)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	history := []v1.History{
		{Comment: message},
	}
	config := v1.Image{
		Config:  configChanges.ImageConfig,
		History: history,
	}
	newImage, err := i.Runtime.ImageRuntime().Import(getContext(), source, reference, nil, image.SigningOptions{}, config)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	if delete {
		if err := os.Remove(source); err != nil {
			return call.ReplyErrorOccurred(err.Error())
		}
	}

	return call.ReplyImportImage(newImage.ID())
}

// ExportImage exports an image to the provided destination
// destination must have the transport type!!
func (i *VarlinkAPI) ExportImage(call iopodman.VarlinkCall, name, destination string, compress bool, tags []string) error {
	newImage, err := i.Runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		return call.ReplyImageNotFound(name, err.Error())
	}

	additionalTags, err := image.GetAdditionalTags(tags)
	if err != nil {
		return err
	}

	if err := newImage.PushImageToHeuristicDestination(getContext(), destination, "", "", "", "", nil, compress, image.SigningOptions{}, &image.DockerRegistryOptions{}, additionalTags); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyExportImage(newImage.ID())
}

// PullImage pulls an image from a registry to the image store.
func (i *VarlinkAPI) PullImage(call iopodman.VarlinkCall, name string, creds iopodman.AuthConfig) error {
	var (
		imageID string
		err     error
	)
	dockerRegistryOptions := image.DockerRegistryOptions{
		DockerRegistryCreds: &types.DockerAuthConfig{
			Username: creds.Username,
			Password: creds.Password,
		},
	}

	so := image.SigningOptions{}

	if call.WantsMore() {
		call.Continues = true
	}
	output := channel.NewWriter(make(chan []byte))
	channelClose := func() {
		if err := output.Close(); err != nil {
			logrus.Errorf("failed to close channel writer: %q", err)
		}
	}
	defer channelClose()
	c := make(chan error)
	defer close(c)

	go func() {
		var foundError bool
		if strings.HasPrefix(name, dockerarchive.Transport.Name()+":") {
			srcRef, err := alltransports.ParseImageName(name)
			if err != nil {
				c <- errors.Wrapf(err, "error parsing %q", name)
			}
			newImage, err := i.Runtime.ImageRuntime().LoadFromArchiveReference(getContext(), srcRef, "", output)
			if err != nil {
				foundError = true
				c <- errors.Wrapf(err, "error pulling image from %q", name)
			} else {
				imageID = newImage[0].ID()
			}
		} else {
			newImage, err := i.Runtime.ImageRuntime().New(getContext(), name, "", "", output, &dockerRegistryOptions, so, nil, util.PullImageMissing)
			if err != nil {
				foundError = true
				c <- errors.Wrapf(err, "unable to pull %s", name)
			} else {
				imageID = newImage.ID()
			}
		}
		if !foundError {
			c <- nil
		}
	}()

	var log []string
	reply := func(br iopodman.MoreResponse) error {
		return call.ReplyPullImage(br)
	}
	log, err = forwardOutput(log, c, call.WantsMore(), output, reply)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	call.Continues = false
	br := iopodman.MoreResponse{
		Logs: log,
		Id:   imageID,
	}
	return call.ReplyPullImage(br)
}

// ImageExists returns bool as to whether the input image exists in local storage
func (i *VarlinkAPI) ImageExists(call iopodman.VarlinkCall, name string) error {
	_, err := i.Runtime.ImageRuntime().NewFromLocal(name)
	if errors.Cause(err) == image.ErrNoSuchImage {
		return call.ReplyImageExists(1)
	}
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyImageExists(0)
}

// ContainerRunlabel ...
func (i *VarlinkAPI) ContainerRunlabel(call iopodman.VarlinkCall, input iopodman.Runlabel) error {
	ctx := getContext()
	dockerRegistryOptions := image.DockerRegistryOptions{}
	stdErr := os.Stderr
	stdOut := os.Stdout
	stdIn := os.Stdin

	runLabel, imageName, err := GetRunlabel(input.Label, input.Image, ctx, i.Runtime, input.Pull, "", dockerRegistryOptions, input.Authfile, "", nil)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	if runLabel == "" {
		return call.ReplyErrorOccurred(fmt.Sprintf("%s does not contain the label %s", input.Image, input.Label))
	}

	cmd, env, err := GenerateRunlabelCommand(runLabel, imageName, input.Name, input.Opts, input.ExtraArgs, "")
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	if err := utils.ExecCmdWithStdStreams(stdIn, stdOut, stdErr, env, cmd[0], cmd[1:]...); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyContainerRunlabel()
}

// ImagesPrune ....
func (i *VarlinkAPI) ImagesPrune(call iopodman.VarlinkCall, all bool, filter []string) error {
	prunedImages, err := i.Runtime.ImageRuntime().PruneImages(context.TODO(), all, []string{})
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyImagesPrune(prunedImages)
}

// ImageSave ....
func (i *VarlinkAPI) ImageSave(call iopodman.VarlinkCall, options iopodman.ImageSaveOptions) error {
	newImage, err := i.Runtime.ImageRuntime().NewFromLocal(options.Name)
	if err != nil {
		if errors.Cause(err) == define.ErrNoSuchImage {
			return call.ReplyImageNotFound(options.Name, err.Error())
		}
		return call.ReplyErrorOccurred(err.Error())
	}

	// Determine if we are dealing with a tarball or dir
	var output string
	outputToDir := false
	if options.Format == "oci-archive" || options.Format == "docker-archive" {
		tempfile, err := ioutil.TempFile("", "varlink_send")
		if err != nil {
			return call.ReplyErrorOccurred(err.Error())
		}
		output = tempfile.Name()
		tempfile.Close()
	} else {
		var err error
		outputToDir = true
		output, err = ioutil.TempDir("", "varlink_send")
		if err != nil {
			return call.ReplyErrorOccurred(err.Error())
		}
	}
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	if call.WantsMore() {
		call.Continues = true
	}

	saveOutput := bytes.NewBuffer([]byte{})
	c := make(chan error)
	go func() {
		err := newImage.Save(getContext(), options.Name, options.Format, output, options.MoreTags, options.Quiet, options.Compress, true)
		c <- err
		close(c)
	}()
	var log []string
	done := false
	for {
		line, err := saveOutput.ReadString('\n')
		if err == nil {
			log = append(log, line)
			continue
		} else if err == io.EOF {
			select {
			case err := <-c:
				if err != nil {
					logrus.Errorf("reading of output during save failed for %s", newImage.ID())
					return call.ReplyErrorOccurred(err.Error())
				}
				done = true
			default:
				if !call.WantsMore() {
					break
				}
				br := iopodman.MoreResponse{
					Logs: log,
				}
				call.ReplyImageSave(br)
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

	sendfile := output
	// Image has been saved to `output`
	if outputToDir {
		// If the output is a directory, we need to tar up the directory to send it back
		// Create a tempfile for the directory tarball
		outputFile, err := ioutil.TempFile("", "varlink_save_dir")
		if err != nil {
			return err
		}
		defer outputFile.Close()
		if err := utils.TarToFilesystem(output, outputFile); err != nil {
			return call.ReplyErrorOccurred(err.Error())
		}
		sendfile = outputFile.Name()
	}
	br := iopodman.MoreResponse{
		Logs: log,
		Id:   sendfile,
	}
	return call.ReplyPushImage(br)
}

// LoadImage ...
func (i *VarlinkAPI) LoadImage(call iopodman.VarlinkCall, name, inputFile string, deleteInputFile, quiet bool) error {
	var (
		names  string
		writer io.Writer
		err    error
	)
	if !quiet {
		writer = os.Stderr
	}

	if call.WantsMore() {
		call.Continues = true
	}
	output := bytes.NewBuffer([]byte{})

	c := make(chan error)
	go func() {
		names, err = i.Runtime.LoadImage(getContext(), name, inputFile, writer, "")
		c <- err
		close(c)
	}()

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
					logrus.Error(err)
					return call.ReplyErrorOccurred(err.Error())
				}
				done = true
			default:
				if !call.WantsMore() {
					break
				}
				br := iopodman.MoreResponse{
					Logs: log,
				}
				call.ReplyLoadImage(br)
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

	br := iopodman.MoreResponse{
		Logs: log,
		Id:   names,
	}
	if deleteInputFile {
		if err := os.Remove(inputFile); err != nil {
			logrus.Errorf("unable to delete input file %s", inputFile)
		}
	}
	return call.ReplyLoadImage(br)
}

// Diff ...
func (i *VarlinkAPI) Diff(call iopodman.VarlinkCall, name string) error {
	var response []iopodman.DiffInfo
	changes, err := i.Runtime.GetDiff("", name)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	for _, change := range changes {
		response = append(response, iopodman.DiffInfo{Path: change.Path, ChangeType: change.Kind.String()})
	}
	return call.ReplyDiff(response)
}

// GetLayersMapWithImageInfo is a development only endpoint to obtain layer information for an image.
func (i *VarlinkAPI) GetLayersMapWithImageInfo(call iopodman.VarlinkCall) error {
	layerInfo, err := image.GetLayersMapWithImageInfo(i.Runtime.ImageRuntime())
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	b, err := json.Marshal(layerInfo)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyGetLayersMapWithImageInfo(string(b))
}

// BuildImageHierarchyMap ...
func (i *VarlinkAPI) BuildImageHierarchyMap(call iopodman.VarlinkCall, name string) error {
	img, err := i.Runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	imageInfo := &image.InfoImage{
		ID:   img.ID(),
		Tags: img.Names(),
	}
	layerInfo, err := image.GetLayersMapWithImageInfo(i.Runtime.ImageRuntime())
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	if err := image.BuildImageHierarchyMap(imageInfo, layerInfo, img.TopLayer()); err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	b, err := json.Marshal(imageInfo)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyBuildImageHierarchyMap(string(b))
}

// ImageTree returns the image tree string for the provided image name or ID
func (i *VarlinkAPI) ImageTree(call iopodman.VarlinkCall, nameOrID string, whatRequires bool) error {
	img, err := i.Runtime.ImageRuntime().NewFromLocal(nameOrID)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}

	tree, err := img.GenerateTree(whatRequires)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	return call.ReplyImageTree(tree)
}
