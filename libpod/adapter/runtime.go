// +build !remoteclient

package adapter

import (
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	"github.com/urfave/cli"
)

// LocalRuntime describes a typical libpod runtime
type LocalRuntime struct {
	Runtime *libpod.Runtime
	Remote  bool
}

// ContainerImage ...
type ContainerImage struct {
	*image.Image
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
