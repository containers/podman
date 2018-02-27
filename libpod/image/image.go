package image

import (
	"fmt"
	"io"
	"os"

	"github.com/containers/image/docker/reference"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
	"github.com/projectatomic/libpod/pkg/inspect"
)

// Image is the primary struct for dealing with images
// It is still very much a work in progress
type Image struct {
	inspect.ImageData
	InputName string
	Local     bool
	runtime   *libpod.Runtime
	image     *storage.Image
}

// NewFromLocal creates a new image object that is intended
// to only deal with local images already in the store (or
// its aliases)
func NewFromLocal(name string, runtime *libpod.Runtime) (Image, error) {
	image := Image{
		InputName: name,
		Local:     true,
		runtime:   runtime,
	}
	localImage, err := image.getLocalImage()
	if err != nil {
		return Image{}, err
	}
	image.image = localImage
	return image, nil
}

// New creates a new image object where the image could be local
// or remote
func New(name string, runtime *libpod.Runtime) (Image, error) {
	// We don't know if the image is local or not ... check local first
	newImage := Image{
		InputName: name,
		Local:     false,
		runtime:   runtime,
	}
	localImage, err := newImage.getLocalImage()
	if err == nil {
		newImage.Local = true
		newImage.image = localImage
		return newImage, nil
	}

	// The image is not local
	pullNames, err := newImage.createNamesToPull()
	if err != nil {
		return newImage, err
	}
	if len(pullNames) == 0 {
		return newImage, errors.Errorf("unable to pull %s", newImage.InputName)
	}
	var writer io.Writer
	writer = os.Stderr
	for _, p := range pullNames {
		_, err := newImage.pull(p, writer, runtime)
		if err == nil {
			newImage.InputName = p
			img, err := newImage.getLocalImage()
			newImage.image = img
			return newImage, err
		}
	}
	return newImage, errors.Errorf("unable to find %s", name)
}

// getLocalImage resolves an unknown input describing an image and
// returns a storage.Image or an error. It is used by NewFromLocal.
func (i *Image) getLocalImage() (*storage.Image, error) {
	imageError := fmt.Sprintf("unable to find '%s' in local storage\n", i.InputName)
	if i.InputName == "" {
		return nil, errors.Errorf("input name is blank")
	}
	var taggedName string
	img, err := i.runtime.GetImage(i.InputName)
	if err == nil {
		return img, err
	}

	// container-storage wasn't able to find it in its current form
	// check if the input name has a tag, and if not, run it through
	// again
	decomposedImage, err := decompose(i.InputName)
	if err != nil {
		return nil, err
	}
	// the inputname isn't tagged, so we assume latest and try again
	if !decomposedImage.isTagged {
		taggedName = fmt.Sprintf("%s:latest", i.InputName)
		img, err = i.runtime.GetImage(taggedName)
		if err == nil {
			return img, nil
		}
	}
	hasReg, err := i.hasRegistry()
	if err != nil {
		return nil, errors.Wrapf(err, imageError)
	}

	// if the input name has a registry in it, the image isnt here
	if hasReg {
		return nil, errors.Errorf("%s", imageError)
	}

	// grab all the local images
	images, err := i.runtime.GetImages(&libpod.ImageFilterParams{})
	if err != nil {
		return nil, err
	}

	// check the repotags of all images for a match
	repoImage, err := findImageInRepotags(decomposedImage, images)
	if err == nil {
		return repoImage, nil
	}

	return nil, errors.Errorf("%s", imageError)
}

// hasRegistry returns a bool/err response if the image has a registry in its
// name
func (i *Image) hasRegistry() (bool, error) {
	imgRef, err := reference.Parse(i.InputName)
	if err != nil {
		return false, err
	}
	registry := reference.Domain(imgRef.(reference.Named))
	if registry != "" {
		return true, nil
	}
	return false, nil
}

// ID returns the image ID as a string
func (i *Image) ID() string {
	return i.image.ID
}

// createNamesToPull looks at a decomposed image and determines the possible
// images names to try pulling in combination with the registries.conf file as well
func (i *Image) createNamesToPull() ([]string, error) {
	var pullNames []string
	decomposedImage, err := decompose(i.InputName)
	if err != nil {
		return nil, err
	}

	if decomposedImage.hasRegistry {
		pullNames = append(pullNames, i.InputName)
	} else {
		registries, err := libpod.GetRegistries()
		if err != nil {
			return nil, err
		}
		for _, registry := range registries {
			decomposedImage.registry = registry
			pullNames = append(pullNames, decomposedImage.assemble())
		}
	}
	return pullNames, nil
}

// pull is a temporary function for stage1 to be able to pull images during the image
// resolution tests.  it will be replaced in stage2 with a more robust function.
func (i *Image) pull(name string, writer io.Writer, r *libpod.Runtime) (string, error) {
	options := libpod.CopyOptions{
		Writer:              writer,
		SignaturePolicyPath: r.GetConfig().SignaturePolicyPath,
	}
	return i.runtime.PullImage(name, options)
}

// Remove an image
// This function is only complete enough for the stage 1 tests.
func (i *Image) Remove(force bool) error {
	_, err := i.runtime.RemoveImage(i.image, force)
	return err
}
