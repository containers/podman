// +build remoteclient

package adapter

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/containers/buildah/imagebuildah"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/types"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/varlink"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/events"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/utils"
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

	return &LocalRuntime{
		&RemoteRuntime{
			Conn:   conn,
			Remote: true,
		},
	}, nil
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
	var iid string
	// TODO We need to find a way to leak certDir, creds, and the tlsverify into this function, normally this would
	// come from cli options but we don't want want those in here either.
	tlsverify := true
	reply, err := iopodman.PullImage().Send(r.Conn, varlink.More, srcRef.DockerReference().String(), "", "", signaturePolicyPath, &tlsverify)
	if err != nil {
		return nil, err
	}

	for {
		responses, flags, err := reply()
		if err != nil {
			return nil, err
		}
		for _, line := range responses.Logs {
			fmt.Print(line)
		}
		iid = responses.Id
		if flags&varlink.Continues == 0 {
			break
		}
	}

	newImage, err := r.NewImageFromLocal(iid)
	if err != nil {
		return nil, err
	}
	return []*ContainerImage{newImage}, nil
}

// New calls into local storage to look for an image in local storage or to pull it
func (r *LocalRuntime) New(ctx context.Context, name, signaturePolicyPath, authfile string, writer io.Writer, dockeroptions *image.DockerRegistryOptions, signingoptions image.SigningOptions, forcePull bool, label *string) (*ContainerImage, error) {
	var iid string
	if label != nil {
		return nil, errors.New("the remote client function does not support checking a remote image for a label")
	}
	var (
		tlsVerify    bool
		tlsVerifyPtr *bool
	)
	if dockeroptions.DockerInsecureSkipTLSVerify == types.OptionalBoolFalse {
		tlsVerify = true
		tlsVerifyPtr = &tlsVerify

	}
	if dockeroptions.DockerInsecureSkipTLSVerify == types.OptionalBoolTrue {
		tlsVerify = false
		tlsVerifyPtr = &tlsVerify
	}

	reply, err := iopodman.PullImage().Send(r.Conn, varlink.More, name, dockeroptions.DockerCertPath, "", signaturePolicyPath, tlsVerifyPtr)
	if err != nil {
		return nil, err
	}
	for {
		responses, flags, err := reply()
		if err != nil {
			return nil, err
		}
		for _, line := range responses.Logs {
			fmt.Print(line)
		}
		iid = responses.Id
		if flags&varlink.Continues == 0 {
			break
		}
	}
	newImage, err := r.NewImageFromLocal(iid)
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
	return r.GetFileFromRemoteHost(tempPath, path, true)
}

func (r *LocalRuntime) GetFileFromRemoteHost(remoteFilePath, outputPath string, delete bool) error {
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	writer := bufio.NewWriter(outputFile)
	defer writer.Flush()

	reply, err := iopodman.ReceiveFile().Send(r.Conn, varlink.Upgrade, remoteFilePath, delete)
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
	outputFile, err := ioutil.TempFile("", "varlink_tar_send")
	if err != nil {
		return err
	}
	defer outputFile.Close()
	defer os.Remove(outputFile.Name())

	// Create the tarball of the context dir to a tempfile
	if err := utils.TarToFilesystem(options.ContextDirectory, outputFile); err != nil {
		return err
	}
	// Send the context dir tarball over varlink.
	tempFile, err := r.SendFileOverVarlink(outputFile.Name())
	if err != nil {
		return err
	}
	buildinfo.ContextDir = tempFile

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

	return strings.Replace(tempFile, ":", "", -1), nil
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
func (r *LocalRuntime) RemoveContainer(ctx context.Context, c *libpod.Container, force, volumes bool) error {
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

	var (
		tls       *bool
		tlsVerify bool
	)
	if dockerRegistryOptions.DockerInsecureSkipTLSVerify == types.OptionalBoolTrue {
		tlsVerify = false
		tls = &tlsVerify
	}
	if dockerRegistryOptions.DockerInsecureSkipTLSVerify == types.OptionalBoolFalse {
		tlsVerify = true
		tls = &tlsVerify
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

// SaveImage is a wrapper function for saving an image to the local filesystem
func (r *LocalRuntime) SaveImage(ctx context.Context, c *cliconfig.SaveValues) error {
	source := c.InputArgs[0]
	additionalTags := c.InputArgs[1:]

	options := iopodman.ImageSaveOptions{
		Name:     source,
		Format:   c.Format,
		Output:   c.Output,
		MoreTags: additionalTags,
		Quiet:    c.Quiet,
		Compress: c.Compress,
	}
	reply, err := iopodman.ImageSave().Send(r.Conn, varlink.More, options)
	if err != nil {
		return err
	}

	var fetchfile string
	for {
		responses, flags, err := reply()
		if err != nil {
			return err
		}
		if len(responses.Id) > 0 {
			fetchfile = responses.Id
		}
		for _, line := range responses.Logs {
			fmt.Print(line)
		}
		if flags&varlink.Continues == 0 {
			break
		}

	}
	if err != nil {
		return err
	}

	outputToDir := false
	outfile := c.Output
	var outputFile *os.File
	// If the result is supposed to be a dir, then we need to put the tarfile
	// from the host in a temporary file
	if options.Format != "oci-archive" && options.Format != "docker-archive" {
		outputToDir = true
		outputFile, err = ioutil.TempFile("", "saveimage_tempfile")
		if err != nil {
			return err
		}
		outfile = outputFile.Name()
		defer outputFile.Close()
		defer os.Remove(outputFile.Name())
	}
	// We now need to fetch the tarball result back to the more system
	if err := r.GetFileFromRemoteHost(fetchfile, outfile, true); err != nil {
		return err
	}

	// If the result is a tarball, we're done
	// If it is a dir, we need to untar the temporary file into the dir
	if outputToDir {
		if err := utils.UntarToFileSystem(c.Output, outputFile, &archive.TarOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// LoadImage loads a container image from a remote client's filesystem
func (r *LocalRuntime) LoadImage(ctx context.Context, name string, cli *cliconfig.LoadValues) (string, error) {
	var names string
	remoteTempFile, err := r.SendFileOverVarlink(cli.Input)
	if err != nil {
		return "", nil
	}
	more := varlink.More
	if cli.Quiet {
		more = 0
	}
	reply, err := iopodman.LoadImage().Send(r.Conn, uint64(more), name, remoteTempFile, cli.Quiet, true)
	if err != nil {
		return "", err
	}

	for {
		responses, flags, err := reply()
		if err != nil {
			logrus.Error(err)
			return "", err
		}
		for _, line := range responses.Logs {
			fmt.Print(line)
		}
		names = responses.Id
		if flags&varlink.Continues == 0 {
			break
		}
	}
	return names, nil
}

// IsImageNotFound checks if the error indicates that no image was found.
func IsImageNotFound(err error) bool {
	if errors.Cause(err) == image.ErrNoSuchImage {
		return true
	}
	switch err.(type) {
	case *iopodman.ImageNotFound:
		return true
	}
	return false
}

// HealthCheck executes a container's healthcheck over a varlink connection
func (r *LocalRuntime) HealthCheck(c *cliconfig.HealthCheckValues) (libpod.HealthCheckStatus, error) {
	return -1, libpod.ErrNotImplemented
}

// JoinOrCreateRootlessPod joins the specified pod if it is running or it creates a new user namespace
// if the pod is stopped
func (r *LocalRuntime) JoinOrCreateRootlessPod(pod *Pod) (bool, int, error) {
	// Nothing to do in the remote case
	return true, 0, nil
}

// Events monitors libpod/podman events over a varlink connection
func (r *LocalRuntime) Events(c *cliconfig.EventValues) error {
	reply, err := iopodman.GetEvents().Send(r.Conn, uint64(varlink.More), c.Filter, c.Since, c.Stream, c.Until)
	if err != nil {
		return errors.Wrapf(err, "unable to obtain events")
	}

	w := bufio.NewWriter(os.Stdout)
	tmpl, err := template.New("events").Parse(c.Format)
	if err != nil {
		return err
	}

	for {
		returnedEvent, flags, err := reply()
		if err != nil {
			// When the error handling is back into podman, we can flip this to a better way to check
			// for problems. For now, this works.
			return err
		}
		if returnedEvent.Time == "" && returnedEvent.Status == "" && returnedEvent.Type == "" {
			// We got a blank event return, signals end of stream in certain cases
			break
		}
		eTime, err := time.Parse(time.RFC3339Nano, returnedEvent.Time)
		if err != nil {
			return errors.Wrapf(err, "unable to parse time of event %s", returnedEvent.Time)
		}
		eType, err := events.StringToType(returnedEvent.Type)
		if err != nil {
			return err
		}
		eStatus, err := events.StringToStatus(returnedEvent.Status)
		if err != nil {
			return err
		}
		event := events.Event{
			ID:     returnedEvent.Id,
			Image:  returnedEvent.Image,
			Name:   returnedEvent.Name,
			Status: eStatus,
			Time:   eTime,
			Type:   eType,
		}
		if len(c.Format) > 0 {
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
		if flags&varlink.Continues == 0 {
			break
		}
	}
	return nil
}
