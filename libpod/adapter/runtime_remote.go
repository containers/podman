// +build remoteclient

package adapter

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/containers/buildah/imagebuildah"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/types"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/storage/pkg/archive"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/varlink/go/varlink"
)

// ImageRuntime is wrapper for image runtime
type RemoteImageRuntime struct{}

// RemoteRuntime describes a wrapper runtime struct
type RemoteRuntime struct {
	Conn   *varlink.Connection
	Remote bool
}

// LocalRuntime describes a typical libpod runtime
type LocalRuntime struct {
	*RemoteRuntime
}

// GetRuntime returns a LocalRuntime struct with the actual runtime embedded in it
func GetRuntime(c *cliconfig.PodmanCommand) (*LocalRuntime, error) {
	runtime := RemoteRuntime{}
	conn, err := runtime.Connect()
	if err != nil {
		return nil, err
	}
	rr := RemoteRuntime{
		Conn:   conn,
		Remote: true,
	}
	foo := LocalRuntime{
		&rr,
	}
	return &foo, nil
}

// Shutdown is a bogus wrapper for compat with the libpod runtime
func (r RemoteRuntime) Shutdown(force bool) error {
	return nil
}

// ContainerImage
type ContainerImage struct {
	remoteImage
}

type remoteImage struct {
	ID          string
	Labels      map[string]string
	RepoTags    []string
	RepoDigests []string
	Parent      string
	Size        int64
	Created     time.Time
	InputName   string
	Names       []string
	Digest      digest.Digest
	isParent    bool
	Runtime     *LocalRuntime
}

// Container ...
type Container struct {
	remoteContainer
}

// remoteContainer ....
type remoteContainer struct {
	Runtime *LocalRuntime
	config  *libpod.ContainerConfig
	state   *libpod.ContainerState
}

type VolumeFilter func(*Volume) bool

// Volume is embed for libpod volumes
type Volume struct {
	remoteVolume
}

type remoteVolume struct {
	Runtime *LocalRuntime
	config  *libpod.VolumeConfig
}

// GetImages returns a slice of containerimages over a varlink connection
func (r *LocalRuntime) GetImages() ([]*ContainerImage, error) {
	var newImages []*ContainerImage
	images, err := iopodman.ListImages().Call(r.Conn)
	if err != nil {
		return nil, err
	}
	for _, i := range images {
		name := i.Id
		if len(i.RepoTags) > 1 {
			name = i.RepoTags[0]
		}
		newImage, err := imageInListToContainerImage(i, name, r)
		if err != nil {
			return nil, err
		}
		newImages = append(newImages, newImage)
	}
	return newImages, nil
}

func imageInListToContainerImage(i iopodman.Image, name string, runtime *LocalRuntime) (*ContainerImage, error) {
	created, err := time.ParseInLocation(time.RFC3339, i.Created, time.UTC)
	if err != nil {
		return nil, err
	}
	ri := remoteImage{
		InputName:   name,
		ID:          i.Id,
		Labels:      i.Labels,
		RepoTags:    i.RepoTags,
		RepoDigests: i.RepoTags,
		Parent:      i.ParentId,
		Size:        i.Size,
		Created:     created,
		Names:       i.RepoTags,
		isParent:    i.IsParent,
		Runtime:     runtime,
	}
	return &ContainerImage{ri}, nil
}

// NewImageFromLocal returns a container image representation of a image over varlink
func (r *LocalRuntime) NewImageFromLocal(name string) (*ContainerImage, error) {
	img, err := iopodman.GetImage().Call(r.Conn, name)
	if err != nil {
		return nil, err
	}
	return imageInListToContainerImage(img, name, r)

}

// LoadFromArchiveReference creates an image from a local archive
func (r *LocalRuntime) LoadFromArchiveReference(ctx context.Context, srcRef types.ImageReference, signaturePolicyPath string, writer io.Writer) ([]*ContainerImage, error) {
	// TODO We need to find a way to leak certDir, creds, and the tlsverify into this function, normally this would
	// come from cli options but we don't want want those in here either.
	imageID, err := iopodman.PullImage().Call(r.Conn, srcRef.DockerReference().String(), "", "", signaturePolicyPath, true)
	if err != nil {
		return nil, err
	}
	newImage, err := r.NewImageFromLocal(imageID)
	if err != nil {
		return nil, err
	}
	return []*ContainerImage{newImage}, nil
}

// New calls into local storage to look for an image in local storage or to pull it
func (r *LocalRuntime) New(ctx context.Context, name, signaturePolicyPath, authfile string, writer io.Writer, dockeroptions *image.DockerRegistryOptions, signingoptions image.SigningOptions, forcePull bool, label *string) (*ContainerImage, error) {
	if label != nil {
		return nil, errors.New("the remote client function does not support checking a remote image for a label")
	}
	// TODO Creds needs to be figured out here too, like above
	tlsBool := dockeroptions.DockerInsecureSkipTLSVerify
	// Remember SkipTlsVerify is the opposite of tlsverify
	// If tlsBook is true or undefined, we do not skip
	SkipTlsVerify := false
	if tlsBool == types.OptionalBoolFalse {
		SkipTlsVerify = true
	}
	imageID, err := iopodman.PullImage().Call(r.Conn, name, dockeroptions.DockerCertPath, "", signaturePolicyPath, SkipTlsVerify)
	if err != nil {
		return nil, err
	}
	newImage, err := r.NewImageFromLocal(imageID)
	if err != nil {
		return nil, err
	}
	return newImage, nil
}

// IsParent goes through the layers in the store and checks if i.TopLayer is
// the parent of any other layer in store. Double check that image with that
// layer exists as well.
func (ci *ContainerImage) IsParent() (bool, error) {
	return ci.remoteImage.isParent, nil
}

// ID returns the image ID as a string
func (ci *ContainerImage) ID() string {
	return ci.remoteImage.ID
}

// Names returns a string array of names associated with the image
func (ci *ContainerImage) Names() []string {
	return ci.remoteImage.Names
}

// Created returns the time the image was created
func (ci *ContainerImage) Created() time.Time {
	return ci.remoteImage.Created
}

// Size returns the size of the image
func (ci *ContainerImage) Size(ctx context.Context) (*uint64, error) {
	usize := uint64(ci.remoteImage.Size)
	return &usize, nil
}

// Digest returns the image's digest
func (ci *ContainerImage) Digest() digest.Digest {
	return ci.remoteImage.Digest
}

// Labels returns a map of the image's labels
func (ci *ContainerImage) Labels(ctx context.Context) (map[string]string, error) {
	return ci.remoteImage.Labels, nil
}

// Dangling returns a bool if the image is "dangling"
func (ci *ContainerImage) Dangling() bool {
	return len(ci.Names()) == 0
}

// TagImage ...
func (ci *ContainerImage) TagImage(tag string) error {
	_, err := iopodman.TagImage().Call(ci.Runtime.Conn, ci.ID(), tag)
	return err
}

// RemoveImage calls varlink to remove an image
func (r *LocalRuntime) RemoveImage(ctx context.Context, img *ContainerImage, force bool) (string, error) {
	return iopodman.RemoveImage().Call(r.Conn, img.InputName, force)
}

// History returns the history of an image and its layers
func (ci *ContainerImage) History(ctx context.Context) ([]*image.History, error) {
	var imageHistories []*image.History

	reply, err := iopodman.HistoryImage().Call(ci.Runtime.Conn, ci.InputName)
	if err != nil {
		return nil, err
	}
	for _, h := range reply {
		created, err := time.ParseInLocation(time.RFC3339, h.Created, time.UTC)
		if err != nil {
			return nil, err
		}
		ih := image.History{
			ID:        h.Id,
			Created:   &created,
			CreatedBy: h.CreatedBy,
			Size:      h.Size,
			Comment:   h.Comment,
		}
		imageHistories = append(imageHistories, &ih)
	}
	return imageHistories, nil
}

// LookupContainer gets basic information about container over a varlink
// connection and then translates it to a *Container
func (r *LocalRuntime) LookupContainer(idOrName string) (*Container, error) {
	state, err := r.ContainerState(idOrName)
	if err != nil {
		return nil, err
	}
	config := r.Config(idOrName)
	if err != nil {
		return nil, err
	}

	rc := remoteContainer{
		r,
		config,
		state,
	}

	c := Container{
		rc,
	}
	return &c, nil
}

func (r *LocalRuntime) GetLatestContainer() (*Container, error) {
	return nil, libpod.ErrNotImplemented
}

// ContainerState returns the "state" of the container.
func (r *LocalRuntime) ContainerState(name string) (*libpod.ContainerState, error) { //no-lint
	reply, err := iopodman.ContainerStateData().Call(r.Conn, name)
	if err != nil {
		return nil, err
	}
	data := libpod.ContainerState{}
	if err := json.Unmarshal([]byte(reply), &data); err != nil {
		return nil, err
	}
	return &data, err

}

// Config returns a container config
func (r *LocalRuntime) Config(name string) *libpod.ContainerConfig {
	// TODO the Spec being returned is not populated.  Matt and I could not figure out why.  Will defer
	// further looking into it for after devconf.
	// The libpod function for this has no errors so we are kind of in a tough
	// spot here.  Logging the errors for now.
	reply, err := iopodman.ContainerConfig().Call(r.Conn, name)
	if err != nil {
		logrus.Error("call to container.config failed")
	}
	data := libpod.ContainerConfig{}
	if err := json.Unmarshal([]byte(reply), &data); err != nil {
		logrus.Error("failed to unmarshal container inspect data")
	}
	return &data

}

// PruneImages is the wrapper call for a remote-client to prune images
func (r *LocalRuntime) PruneImages(all bool) ([]string, error) {
	return iopodman.ImagesPrune().Call(r.Conn, all)
}

// Export is a wrapper to container export to a tarfile
func (r *LocalRuntime) Export(name string, path string) error {
	tempPath, err := iopodman.ExportContainer().Call(r.Conn, name, "")
	if err != nil {
		return err
	}

	outputFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	writer := bufio.NewWriter(outputFile)
	defer writer.Flush()

	reply, err := iopodman.ReceiveFile().Send(r.Conn, varlink.Upgrade, tempPath, true)
	if err != nil {
		return err
	}

	length, _, err := reply()
	if err != nil {
		return errors.Wrap(err, "unable to get file length for transfer")
	}

	reader := r.Conn.Reader
	if _, err := io.CopyN(writer, reader, length); err != nil {
		return errors.Wrap(err, "file transer failed")
	}

	return nil
}

// Import implements the remote calls required to import a container image to the store
func (r *LocalRuntime) Import(ctx context.Context, source, reference string, changes []string, history string, quiet bool) (string, error) {
	// First we send the file to the host
	tempFile, err := r.SendFileOverVarlink(source)
	if err != nil {
		return "", err
	}
	return iopodman.ImportImage().Call(r.Conn, strings.TrimRight(tempFile, ":"), reference, history, changes, true)
}

func (r *LocalRuntime) Build(ctx context.Context, c *cliconfig.BuildValues, options imagebuildah.BuildOptions, dockerfiles []string) error {
	buildOptions := iopodman.BuildOptions{
		AddHosts:     options.CommonBuildOpts.AddHost,
		CgroupParent: options.CommonBuildOpts.CgroupParent,
		CpuPeriod:    int64(options.CommonBuildOpts.CPUPeriod),
		CpuQuota:     options.CommonBuildOpts.CPUQuota,
		CpuShares:    int64(options.CommonBuildOpts.CPUShares),
		CpusetCpus:   options.CommonBuildOpts.CPUSetMems,
		CpusetMems:   options.CommonBuildOpts.CPUSetMems,
		Memory:       options.CommonBuildOpts.Memory,
		MemorySwap:   options.CommonBuildOpts.MemorySwap,
		ShmSize:      options.CommonBuildOpts.ShmSize,
		Ulimit:       options.CommonBuildOpts.Ulimit,
		Volume:       options.CommonBuildOpts.Volumes,
	}

	buildinfo := iopodman.BuildInfo{
		AdditionalTags:        options.AdditionalTags,
		Annotations:           options.Annotations,
		BuildArgs:             options.Args,
		BuildOptions:          buildOptions,
		CniConfigDir:          options.CNIConfigDir,
		CniPluginDir:          options.CNIPluginPath,
		Compression:           string(options.Compression),
		DefaultsMountFilePath: options.DefaultMountsFilePath,
		Dockerfiles:           dockerfiles,
		//Err: string(options.Err),
		ForceRmIntermediateCtrs: options.ForceRmIntermediateCtrs,
		Iidfile:                 options.IIDFile,
		Label:                   options.Labels,
		Layers:                  options.Layers,
		Nocache:                 options.NoCache,
		//Out:
		Output:                 options.Output,
		OutputFormat:           options.OutputFormat,
		PullPolicy:             options.PullPolicy.String(),
		Quiet:                  options.Quiet,
		RemoteIntermediateCtrs: options.RemoveIntermediateCtrs,
		//ReportWriter:
		RuntimeArgs:         options.RuntimeArgs,
		SignaturePolicyPath: options.SignaturePolicyPath,
		Squash:              options.Squash,
	}
	// tar the file
	logrus.Debugf("creating tarball of context dir %s", options.ContextDirectory)
	input, err := archive.Tar(options.ContextDirectory, archive.Uncompressed)
	if err != nil {
		return errors.Wrapf(err, "unable to create tarball of context dir %s", options.ContextDirectory)
	}

	// Write the tarball to the fs
	// TODO we might considering sending this without writing to the fs for the sake of performance
	// under given conditions like memory availability.
	outputFile, err := ioutil.TempFile("", "varlink_tar_send")
	if err != nil {
		return err
	}
	defer outputFile.Close()
	logrus.Debugf("writing context dir tarball to %s", outputFile.Name())

	_, err = io.Copy(outputFile, input)
	if err != nil {
		return err
	}

	logrus.Debugf("completed writing context dir tarball %s", outputFile.Name())
	// Send the context dir tarball over varlink.
	tempFile, err := r.SendFileOverVarlink(outputFile.Name())
	if err != nil {
		return err
	}
	buildinfo.ContextDir = strings.Replace(tempFile, ":", "", -1)

	reply, err := iopodman.BuildImage().Send(r.Conn, varlink.More, buildinfo)
	if err != nil {
		return err
	}

	for {
		responses, flags, err := reply()
		if err != nil {
			return err
		}
		for _, line := range responses.Logs {
			fmt.Print(line)
		}
		if flags&varlink.Continues == 0 {
			break
		}
	}
	return err
}

// SendFileOverVarlink sends a file over varlink in an upgraded connection
func (r *LocalRuntime) SendFileOverVarlink(source string) (string, error) {
	fs, err := os.Open(source)
	if err != nil {
		return "", err
	}

	fileInfo, err := fs.Stat()
	if err != nil {
		return "", err
	}
	logrus.Debugf("sending %s over varlink connection", source)
	reply, err := iopodman.SendFile().Send(r.Conn, varlink.Upgrade, "", int64(fileInfo.Size()))
	if err != nil {
		return "", err
	}
	_, _, err = reply()
	if err != nil {
		return "", err
	}

	reader := bufio.NewReader(fs)
	_, err = reader.WriteTo(r.Conn.Writer)
	if err != nil {
		return "", err
	}
	logrus.Debugf("file transfer complete for %s", source)
	r.Conn.Writer.Flush()

	// All was sent, wait for the ACK from the server
	tempFile, err := r.Conn.Reader.ReadString(':')
	if err != nil {
		return "", err
	}

	// r.Conn is kaput at this point due to the upgrade
	if err := r.RemoteRuntime.RefreshConnection(); err != nil {
		return "", err

	}

	return tempFile, nil
}

// GetAllVolumes retrieves all the volumes
func (r *LocalRuntime) GetAllVolumes() ([]*libpod.Volume, error) {
	return nil, libpod.ErrNotImplemented
}

// RemoveVolume removes a volumes
func (r *LocalRuntime) RemoveVolume(ctx context.Context, v *libpod.Volume, force, prune bool) error {
	return libpod.ErrNotImplemented
}

// GetContainers retrieves all containers from the state
// Filters can be provided which will determine what containers are included in
// the output. Multiple filters are handled by ANDing their output, so only
// containers matching all filters are returned
func (r *LocalRuntime) GetContainers(filters ...libpod.ContainerFilter) ([]*libpod.Container, error) {
	return nil, libpod.ErrNotImplemented
}

// RemoveContainer removes the given container
// If force is specified, the container will be stopped first
// Otherwise, RemoveContainer will return an error if the container is running
func (r *LocalRuntime) RemoveContainer(ctx context.Context, c *libpod.Container, force bool) error {
	return libpod.ErrNotImplemented
}

// CreateVolume creates a volume over a varlink connection for the remote client
func (r *LocalRuntime) CreateVolume(ctx context.Context, c *cliconfig.VolumeCreateValues, labels, opts map[string]string) (string, error) {
	cvOpts := iopodman.VolumeCreateOpts{
		Options: opts,
		Labels:  labels,
	}
	if len(c.InputArgs) > 0 {
		cvOpts.VolumeName = c.InputArgs[0]
	}

	if c.Flag("driver").Changed {
		cvOpts.Driver = c.Driver
	}

	return iopodman.VolumeCreate().Call(r.Conn, cvOpts)
}

// RemoveVolumes removes volumes over a varlink connection for the remote client
func (r *LocalRuntime) RemoveVolumes(ctx context.Context, c *cliconfig.VolumeRmValues) ([]string, error) {
	rmOpts := iopodman.VolumeRemoveOpts{
		All:     c.All,
		Force:   c.Force,
		Volumes: c.InputArgs,
	}
	return iopodman.VolumeRemove().Call(r.Conn, rmOpts)
}

func (r *LocalRuntime) Push(ctx context.Context, srcName, destination, manifestMIMEType, authfile, signaturePolicyPath string, writer io.Writer, forceCompress bool, signingOptions image.SigningOptions, dockerRegistryOptions *image.DockerRegistryOptions, additionalDockerArchiveTags []reference.NamedTagged) error {

	tls := true
	if dockerRegistryOptions.DockerInsecureSkipTLSVerify == types.OptionalBoolTrue {
		tls = false
	}
	reply, err := iopodman.PushImage().Send(r.Conn, varlink.More, srcName, destination, tls, signaturePolicyPath, "", dockerRegistryOptions.DockerCertPath, forceCompress, manifestMIMEType, signingOptions.RemoveSignatures, signingOptions.SignBy)
	if err != nil {
		return err
	}
	for {
		responses, flags, err := reply()
		if err != nil {
			return err
		}
		for _, line := range responses.Logs {
			fmt.Print(line)
		}
		if flags&varlink.Continues == 0 {
			break
		}
	}

	return err
}

// InspectVolumes returns a slice of volumes based on an arg list or --all
func (r *LocalRuntime) InspectVolumes(ctx context.Context, c *cliconfig.VolumeInspectValues) ([]*Volume, error) {
	reply, err := iopodman.GetVolumes().Call(r.Conn, c.InputArgs, c.All)
	if err != nil {
		return nil, err
	}
	return varlinkVolumeToVolume(r, reply), nil
}

//Volumes returns a slice of adapter.volumes based on information about libpod
// volumes over a varlink connection
func (r *LocalRuntime) Volumes(ctx context.Context) ([]*Volume, error) {
	reply, err := iopodman.GetVolumes().Call(r.Conn, []string{}, true)
	if err != nil {
		return nil, err
	}
	return varlinkVolumeToVolume(r, reply), nil
}

func varlinkVolumeToVolume(r *LocalRuntime, volumes []iopodman.Volume) []*Volume {
	var vols []*Volume
	for _, v := range volumes {
		volumeConfig := libpod.VolumeConfig{
			Name:       v.Name,
			Labels:     v.Labels,
			MountPoint: v.MountPoint,
			Driver:     v.Driver,
			Options:    v.Options,
			Scope:      v.Scope,
		}
		n := remoteVolume{
			Runtime: r,
			config:  &volumeConfig,
		}
		newVol := Volume{
			n,
		}
		vols = append(vols, &newVol)
	}
	return vols
}

// PruneVolumes removes all unused volumes from the remote system
func (r *LocalRuntime) PruneVolumes(ctx context.Context) ([]string, []error) {
	var errs []error
	prunedNames, prunedErrors, err := iopodman.VolumesPrune().Call(r.Conn)
	if err != nil {
		return []string{}, []error{err}
	}
	// We need to transform the string results of the error into actual error types
	for _, e := range prunedErrors {
		errs = append(errs, errors.New(e))
	}
	return prunedNames, errs
}
