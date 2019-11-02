// +build !remoteclient

package adapter

import (
	"bufio"
	"context"
	"io"
	"io/ioutil"
	"os"
	"text/template"

	"github.com/containers/buildah"
	"github.com/containers/buildah/imagebuildah"
	"github.com/containers/buildah/pkg/formats"
	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/libpod/events"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/storage/pkg/archive"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
)

// LocalRuntime describes a typical libpod runtime
type LocalRuntime struct {
	*libpod.Runtime
	Remote bool
}

// ContainerImage ...
type ContainerImage struct {
	*image.Image
}

// Container ...
type Container struct {
	*libpod.Container
}

// Pod encapsulates the libpod.Pod structure, helps with remote vs. local
type Pod struct {
	*libpod.Pod
}

// Volume ...
type Volume struct {
	*libpod.Volume
}

// VolumeFilter is for filtering volumes on the client
type VolumeFilter func(*Volume) bool

// GetRuntimeNoStore returns a localruntime struct wit an embedded runtime but
// without a configured storage.
func GetRuntimeNoStore(ctx context.Context, c *cliconfig.PodmanCommand) (*LocalRuntime, error) {
	runtime, err := libpodruntime.GetRuntimeNoStore(ctx, c)
	if err != nil {
		return nil, err
	}
	return getRuntime(runtime)
}

// GetRuntime returns a LocalRuntime struct with the actual runtime embedded in it
func GetRuntime(ctx context.Context, c *cliconfig.PodmanCommand) (*LocalRuntime, error) {
	runtime, err := libpodruntime.GetRuntime(ctx, c)
	if err != nil {
		return nil, err
	}
	return getRuntime(runtime)
}

func getRuntime(runtime *libpod.Runtime) (*LocalRuntime, error) {
	return &LocalRuntime{
		Runtime: runtime,
	}, nil
}

// GetImages returns a slice of images in containerimages
func (r *LocalRuntime) GetImages() ([]*ContainerImage, error) {
	return r.getImages(false)
}

// GetRWImages returns a slice of read/write images in containerimages
func (r *LocalRuntime) GetRWImages() ([]*ContainerImage, error) {
	return r.getImages(true)
}

func (r *LocalRuntime) getImages(rwOnly bool) ([]*ContainerImage, error) {
	var containerImages []*ContainerImage
	images, err := r.Runtime.ImageRuntime().GetImages()
	if err != nil {
		return nil, err
	}
	for _, i := range images {
		if rwOnly && i.IsReadOnly() {
			continue
		}
		containerImages = append(containerImages, &ContainerImage{i})
	}
	return containerImages, nil
}

// NewImageFromLocal returns a containerimage representation of a image from local storage
func (r *LocalRuntime) NewImageFromLocal(name string) (*ContainerImage, error) {
	img, err := r.Runtime.ImageRuntime().NewFromLocal(name)
	if err != nil {
		return nil, err
	}
	return &ContainerImage{img}, nil
}

// LoadFromArchiveReference calls into local storage to load an image from an archive
func (r *LocalRuntime) LoadFromArchiveReference(ctx context.Context, srcRef types.ImageReference, signaturePolicyPath string, writer io.Writer) ([]*ContainerImage, error) {
	var containerImages []*ContainerImage
	imgs, err := r.Runtime.ImageRuntime().LoadFromArchiveReference(ctx, srcRef, signaturePolicyPath, writer)
	if err != nil {
		return nil, err
	}
	for _, i := range imgs {
		ci := ContainerImage{i}
		containerImages = append(containerImages, &ci)
	}
	return containerImages, nil
}

// New calls into local storage to look for an image in local storage or to pull it
func (r *LocalRuntime) New(ctx context.Context, name, signaturePolicyPath, authfile string, writer io.Writer, dockeroptions *image.DockerRegistryOptions, signingoptions image.SigningOptions, label *string, pullType util.PullType) (*ContainerImage, error) {
	img, err := r.Runtime.ImageRuntime().New(ctx, name, signaturePolicyPath, authfile, writer, dockeroptions, signingoptions, label, pullType)
	if err != nil {
		return nil, err
	}
	return &ContainerImage{img}, nil
}

// RemoveImage calls into local storage and removes an image
func (r *LocalRuntime) RemoveImage(ctx context.Context, img *ContainerImage, force bool) (string, error) {
	return r.Runtime.RemoveImage(ctx, img.Image, force)
}

// PruneImages is wrapper into PruneImages within the image pkg
func (r *LocalRuntime) PruneImages(ctx context.Context, all bool) ([]string, error) {
	return r.ImageRuntime().PruneImages(ctx, all)
}

// Export is a wrapper to container export to a tarfile
func (r *LocalRuntime) Export(name string, path string) error {
	ctr, err := r.Runtime.LookupContainer(name)
	if err != nil {
		return errors.Wrapf(err, "error looking up container %q", name)
	}
	return ctr.Export(path)
}

// Import is a wrapper to import a container image
func (r *LocalRuntime) Import(ctx context.Context, source, reference string, changes []string, history string, quiet bool) (string, error) {
	return r.Runtime.Import(ctx, source, reference, changes, history, quiet)
}

// CreateVolume is a wrapper to create volumes
func (r *LocalRuntime) CreateVolume(ctx context.Context, c *cliconfig.VolumeCreateValues, labels, opts map[string]string) (string, error) {
	var (
		options []libpod.VolumeCreateOption
		volName string
	)

	if len(c.InputArgs) > 0 {
		volName = c.InputArgs[0]
		options = append(options, libpod.WithVolumeName(volName))
	}

	if c.Flag("driver").Changed {
		options = append(options, libpod.WithVolumeDriver(c.Driver))
	}

	if len(labels) != 0 {
		options = append(options, libpod.WithVolumeLabels(labels))
	}

	if len(opts) != 0 {
		// We need to process -o for uid, gid
		parsedOptions, err := shared.ParseVolumeOptions(opts)
		if err != nil {
			return "", err
		}
		options = append(options, parsedOptions...)
	}
	newVolume, err := r.NewVolume(ctx, options...)
	if err != nil {
		return "", err
	}
	return newVolume.Name(), nil
}

// RemoveVolumes is a wrapper to remove volumes
func (r *LocalRuntime) RemoveVolumes(ctx context.Context, c *cliconfig.VolumeRmValues) ([]string, map[string]error, error) {
	return shared.SharedRemoveVolumes(ctx, r.Runtime, c.InputArgs, c.All, c.Force)
}

// Push is a wrapper to push an image to a registry
func (r *LocalRuntime) Push(ctx context.Context, srcName, destination, manifestMIMEType, authfile, digestfile, signaturePolicyPath string, writer io.Writer, forceCompress bool, signingOptions image.SigningOptions, dockerRegistryOptions *image.DockerRegistryOptions, additionalDockerArchiveTags []reference.NamedTagged) error {
	newImage, err := r.ImageRuntime().NewFromLocal(srcName)
	if err != nil {
		return err
	}
	return newImage.PushImageToHeuristicDestination(ctx, destination, manifestMIMEType, authfile, digestfile, signaturePolicyPath, writer, forceCompress, signingOptions, dockerRegistryOptions, nil)
}

// InspectVolumes returns a slice of volumes based on an arg list or --all
func (r *LocalRuntime) InspectVolumes(ctx context.Context, c *cliconfig.VolumeInspectValues) ([]*libpod.InspectVolumeData, error) {
	var (
		volumes []*libpod.Volume
		err     error
	)

	if c.All {
		volumes, err = r.GetAllVolumes()
	} else {
		for _, v := range c.InputArgs {
			vol, err := r.LookupVolume(v)
			if err != nil {
				return nil, err
			}
			volumes = append(volumes, vol)
		}
	}
	if err != nil {
		return nil, err
	}

	inspectVols := make([]*libpod.InspectVolumeData, 0, len(volumes))
	for _, vol := range volumes {
		inspectOut, err := vol.Inspect()
		if err != nil {
			return nil, errors.Wrapf(err, "error inspecting volume %s", vol.Name())
		}
		inspectVols = append(inspectVols, inspectOut)
	}

	return inspectVols, nil
}

// Volumes returns a slice of localruntime volumes
func (r *LocalRuntime) Volumes(ctx context.Context) ([]*Volume, error) {
	vols, err := r.GetAllVolumes()
	if err != nil {
		return nil, err
	}
	return libpodVolumeToVolume(vols), nil
}

// libpodVolumeToVolume converts a slice of libpod volumes to a slice
// of localruntime volumes (same as libpod)
func libpodVolumeToVolume(volumes []*libpod.Volume) []*Volume {
	var vols []*Volume
	for _, v := range volumes {
		newVol := Volume{
			v,
		}
		vols = append(vols, &newVol)
	}
	return vols
}

// Build is the wrapper to build images
func (r *LocalRuntime) Build(ctx context.Context, c *cliconfig.BuildValues, options imagebuildah.BuildOptions, dockerfiles []string) error {
	namespaceOptions, networkPolicy, err := parse.NamespaceOptions(c.PodmanCommand.Command)
	if err != nil {
		return errors.Wrapf(err, "error parsing namespace-related options")
	}
	usernsOption, idmappingOptions, err := parse.IDMappingOptions(c.PodmanCommand.Command, options.Isolation)
	if err != nil {
		return errors.Wrapf(err, "error parsing ID mapping options")
	}
	namespaceOptions.AddOrReplace(usernsOption...)

	systemContext, err := parse.SystemContextFromOptions(c.PodmanCommand.Command)
	if err != nil {
		return errors.Wrapf(err, "error building system context")
	}

	authfile := c.Authfile
	if len(c.Authfile) == 0 {
		authfile = os.Getenv("REGISTRY_AUTH_FILE")
	}

	systemContext.AuthFilePath = authfile
	commonOpts, err := parse.CommonBuildOptions(c.PodmanCommand.Command)
	if err != nil {
		return err
	}

	options.NamespaceOptions = namespaceOptions
	options.ConfigureNetwork = networkPolicy
	options.IDMappingOptions = idmappingOptions
	options.CommonBuildOpts = commonOpts
	options.SystemContext = systemContext

	if c.GlobalFlags.Runtime != "" {
		options.Runtime = c.GlobalFlags.Runtime
	} else {
		options.Runtime = r.GetOCIRuntimePath()
	}

	if c.Quiet {
		options.ReportWriter = ioutil.Discard
	}

	if rootless.IsRootless() {
		options.Isolation = buildah.IsolationOCIRootless
	}

	return r.Runtime.Build(ctx, options, dockerfiles...)
}

// PruneVolumes is a wrapper function for libpod PruneVolumes
func (r *LocalRuntime) PruneVolumes(ctx context.Context) ([]string, []error) {
	return r.Runtime.PruneVolumes(ctx)
}

// SaveImage is a wrapper function for saving an image to the local filesystem
func (r *LocalRuntime) SaveImage(ctx context.Context, c *cliconfig.SaveValues) error {
	source := c.InputArgs[0]
	additionalTags := c.InputArgs[1:]

	newImage, err := r.Runtime.ImageRuntime().NewFromLocal(source)
	if err != nil {
		return err
	}
	return newImage.Save(ctx, source, c.Format, c.Output, additionalTags, c.Quiet, c.Compress)
}

// LoadImage is a wrapper function for libpod LoadImage
func (r *LocalRuntime) LoadImage(ctx context.Context, name string, cli *cliconfig.LoadValues) (string, error) {
	var (
		writer io.Writer
	)
	if !cli.Quiet {
		writer = os.Stderr
	}
	return r.Runtime.LoadImage(ctx, name, cli.Input, writer, cli.SignaturePolicy)
}

// IsImageNotFound checks if the error indicates that no image was found.
func IsImageNotFound(err error) bool {
	return errors.Cause(err) == image.ErrNoSuchImage
}

// HealthCheck is a wrapper to same named function in libpod
func (r *LocalRuntime) HealthCheck(c *cliconfig.HealthCheckValues) (string, error) {
	output := "unhealthy"
	status, err := r.Runtime.HealthCheck(c.InputArgs[0])
	if status == libpod.HealthCheckSuccess {
		output = "healthy"
	}
	return output, err
}

// Events is a wrapper to libpod to obtain libpod/podman events
func (r *LocalRuntime) Events(c *cliconfig.EventValues) error {
	var (
		fromStart   bool
		eventsError error
	)
	var tmpl *template.Template
	if c.Format != formats.JSONString {
		template, err := template.New("events").Parse(c.Format)
		if err != nil {
			return err
		}
		tmpl = template
	}
	if len(c.Since) > 0 || len(c.Until) > 0 {
		fromStart = true
	}
	eventChannel := make(chan *events.Event)
	go func() {
		readOpts := events.ReadOptions{FromStart: fromStart, Stream: c.Stream, Filters: c.Filter, EventChannel: eventChannel, Since: c.Since, Until: c.Until}
		eventsError = r.Runtime.Events(readOpts)
	}()

	if eventsError != nil {
		return eventsError
	}
	w := bufio.NewWriter(os.Stdout)
	for event := range eventChannel {
		if c.Format == formats.JSONString {
			jsonStr, err := event.ToJSONString()
			if err != nil {
				return errors.Wrapf(err, "unable to format json")
			}
			if _, err := w.Write([]byte(jsonStr)); err != nil {
				return err
			}
		} else if len(c.Format) > 0 {
			if err := tmpl.Execute(w, event); err != nil {
				return err
			}
		} else {
			if _, err := w.Write([]byte(event.ToHumanReadable())); err != nil {
				return err
			}
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return err
		}
		if err := w.Flush(); err != nil {
			return err
		}
	}
	return nil
}

// Diff shows the difference in two objects
func (r *LocalRuntime) Diff(c *cliconfig.DiffValues, to string) ([]archive.Change, error) {
	return r.Runtime.GetDiff("", to)
}

// GenerateKube creates kubernetes email from containers and pods
func (r *LocalRuntime) GenerateKube(c *cliconfig.GenerateKubeValues) (*v1.Pod, *v1.Service, error) {
	return shared.GenerateKube(c.InputArgs[0], c.Service, r.Runtime)
}

// GetPodsByStatus returns a slice of pods filtered by a libpod status
func (r *LocalRuntime) GetPodsByStatus(statuses []string) ([]*libpod.Pod, error) {

	filterFunc := func(p *libpod.Pod) bool {
		state, _ := shared.GetPodStatus(p)
		for _, status := range statuses {
			if state == status {
				return true
			}
		}
		return false
	}

	pods, err := r.Runtime.Pods(filterFunc)
	if err != nil {
		return nil, err
	}

	return pods, nil
}

// GetVersion is an alias to satisfy interface{}
func (r *LocalRuntime) GetVersion() (define.Version, error) {
	return define.GetVersion()
}

// RemoteEndpoint resolve interface requirement
func (r *LocalRuntime) RemoteEndpoint() (*Endpoint, error) {
	return nil, errors.New("RemoteEndpoint() not implemented for local connection")
}
