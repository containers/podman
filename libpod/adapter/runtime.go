// +build !remoteclient

package adapter

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/types"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
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

// GetRuntime returns a LocalRuntime struct with the actual runtime embedded in it
func GetRuntime(c *cliconfig.PodmanCommand) (*LocalRuntime, error) {
	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return nil, err
	}
	return &LocalRuntime{
		Runtime: runtime,
	}, nil
}

// GetImages returns a slice of images in containerimages
func (r *LocalRuntime) GetImages() ([]*ContainerImage, error) {
	var containerImages []*ContainerImage
	images, err := r.Runtime.ImageRuntime().GetImages()
	if err != nil {
		return nil, err
	}
	for _, i := range images {
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
func (r *LocalRuntime) New(ctx context.Context, name, signaturePolicyPath, authfile string, writer io.Writer, dockeroptions *image.DockerRegistryOptions, signingoptions image.SigningOptions, forcePull bool, label *string) (*ContainerImage, error) {
	img, err := r.Runtime.ImageRuntime().New(ctx, name, signaturePolicyPath, authfile, writer, dockeroptions, signingoptions, forcePull, label)
	if err != nil {
		return nil, err
	}
	return &ContainerImage{img}, nil
}

// RemoveImage calls into local storage and removes an image
func (r *LocalRuntime) RemoveImage(ctx context.Context, img *ContainerImage, force bool) (string, error) {
	return r.Runtime.RemoveImage(ctx, img.Image, force)
}

// LookupContainer ...
func (r *LocalRuntime) LookupContainer(idOrName string) (*Container, error) {
	ctr, err := r.Runtime.LookupContainer(idOrName)
	if err != nil {
		return nil, err
	}
	return &Container{ctr}, nil
}

// PruneImages is wrapper into PruneImages within the image pkg
func (r *LocalRuntime) PruneImages(all bool) ([]string, error) {
	return r.ImageRuntime().PruneImages(all)
}

// Export is a wrapper to container export to a tarfile
func (r *LocalRuntime) Export(name string, path string) error {
	ctr, err := r.Runtime.LookupContainer(name)
	if err != nil {
		return errors.Wrapf(err, "error looking up container %q", name)
	}
	if os.Geteuid() != 0 {
		state, err := ctr.State()
		if err != nil {
			return errors.Wrapf(err, "cannot read container state %q", ctr.ID())
		}
		if state == libpod.ContainerStateRunning || state == libpod.ContainerStatePaused {
			data, err := ioutil.ReadFile(ctr.Config().ConmonPidFile)
			if err != nil {
				return errors.Wrapf(err, "cannot read conmon PID file %q", ctr.Config().ConmonPidFile)
			}
			conmonPid, err := strconv.Atoi(string(data))
			if err != nil {
				return errors.Wrapf(err, "cannot parse PID %q", data)
			}
			became, ret, err := rootless.JoinDirectUserAndMountNS(uint(conmonPid))
			if err != nil {
				return err
			}
			if became {
				os.Exit(ret)
			}
		} else {
			became, ret, err := rootless.BecomeRootInUserNS()
			if err != nil {
				return err
			}
			if became {
				os.Exit(ret)
			}
		}
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

	if len(options) != 0 {
		options = append(options, libpod.WithVolumeOptions(opts))
	}
	newVolume, err := r.NewVolume(ctx, options...)
	if err != nil {
		return "", err
	}
	return newVolume.Name(), nil
}

// RemoveVolumes is a wrapper to remove volumes
func (r *LocalRuntime) RemoveVolumes(ctx context.Context, c *cliconfig.VolumeRmValues) ([]string, error) {
	return r.Runtime.RemoveVolumes(ctx, c.InputArgs, c.All, c.Force)
}

// Push is a wrapper to push an image to a registry
func (r *LocalRuntime) Push(ctx context.Context, srcName, destination, manifestMIMEType, authfile, signaturePolicyPath string, writer io.Writer, forceCompress bool, signingOptions image.SigningOptions, dockerRegistryOptions *image.DockerRegistryOptions, additionalDockerArchiveTags []reference.NamedTagged) error {
	newImage, err := r.ImageRuntime().NewFromLocal(srcName)
	if err != nil {
		return err
	}
	return newImage.PushImageToHeuristicDestination(ctx, destination, manifestMIMEType, authfile, signaturePolicyPath, writer, forceCompress, signingOptions, dockerRegistryOptions, nil)
}
