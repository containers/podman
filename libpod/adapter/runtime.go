// +build !remoteclient

package adapter

import (
	"context"
	"io"

	"github.com/containers/image/types"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	"github.com/urfave/cli"
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
func GetRuntime(c *cli.Context) (*LocalRuntime, error) {
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
