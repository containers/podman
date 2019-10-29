package libpod

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"github.com/containers/buildah/imagebuildah"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/containers/image/v5/directory"
	dockerarchive "github.com/containers/image/v5/docker/archive"
	ociarchive "github.com/containers/image/v5/oci/archive"
	"github.com/opencontainers/image-spec/specs-go/v1"
)

// Runtime API

// RemoveImage deletes an image from local storage
// Images being used by running containers can only be removed if force=true
func (r *Runtime) RemoveImage(ctx context.Context, img *image.Image, force bool) (string, error) {
	var returnMessage string
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.valid {
		return "", define.ErrRuntimeStopped
	}

	// Get all containers, filter to only those using the image, and remove those containers
	ctrs, err := r.state.AllContainers()
	if err != nil {
		return "", err
	}
	imageCtrs := []*Container{}
	for _, ctr := range ctrs {
		if ctr.config.RootfsImageID == img.ID() {
			imageCtrs = append(imageCtrs, ctr)
		}
	}
	if len(imageCtrs) > 0 && len(img.Names()) <= 1 {
		if force {
			for _, ctr := range imageCtrs {
				if err := r.removeContainer(ctx, ctr, true, false, false); err != nil {
					return "", errors.Wrapf(err, "error removing image %s: container %s using image could not be removed", img.ID(), ctr.ID())
				}
			}
		} else {
			return "", fmt.Errorf("could not remove image %s as it is being used by %d containers", img.ID(), len(imageCtrs))
		}
	}

	hasChildren, err := img.IsParent(ctx)
	if err != nil {
		return "", err
	}

	if (len(img.Names()) > 1 && !img.InputIsID()) || hasChildren {
		// If the image has multiple reponames, we do not technically delete
		// the image. we figure out which repotag the user is trying to refer
		// to and untag it.
		repoName, err := img.MatchRepoTag(img.InputName)
		if hasChildren && errors.Cause(err) == image.ErrRepoTagNotFound {
			return "", errors.Errorf("unable to delete %q (cannot be forced) - image has dependent child images", img.ID())
		}
		if err != nil {
			return "", err
		}
		if err := img.UntagImage(repoName); err != nil {
			return "", err
		}
		return fmt.Sprintf("Untagged: %s", repoName), nil
	} else if len(img.Names()) > 1 && img.InputIsID() && !force {
		// If the user requests to delete an image by ID and the image has multiple
		// reponames and no force is applied, we error out.
		return "", fmt.Errorf("unable to delete %s (must force) - image is referred to in multiple tags", img.ID())
	}
	err = img.Remove(ctx, force)
	if err != nil && errors.Cause(err) == storage.ErrImageUsedByContainer {
		if errStorage := r.rmStorageContainers(force, img); errStorage == nil {
			// Containers associated with the image should be deleted now,
			// let's try removing the image again.
			err = img.Remove(ctx, force)
		} else {
			err = errStorage
		}
	}
	for _, name := range img.Names() {
		returnMessage = returnMessage + fmt.Sprintf("Untagged: %s\n", name)
	}
	returnMessage = returnMessage + fmt.Sprintf("Deleted: %s", img.ID())
	return returnMessage, err
}

// Remove containers that are in storage rather than Podman.
func (r *Runtime) rmStorageContainers(force bool, image *image.Image) error {
	ctrIDs, err := storageContainers(image.ID(), r.store)
	if err != nil {
		return errors.Wrapf(err, "error getting containers for image %q", image.ID())
	}

	if len(ctrIDs) > 0 && !force {
		return storage.ErrImageUsedByContainer
	}

	if len(ctrIDs) > 0 && force {
		if err = removeStorageContainers(ctrIDs, r.store); err != nil {
			return errors.Wrapf(err, "error removing containers %v for image %q", ctrIDs, image.ID())
		}
	}
	return nil
}

// Returns a list of storage containers associated with the given ImageReference
func storageContainers(imageID string, store storage.Store) ([]string, error) {
	ctrIDs := []string{}
	containers, err := store.Containers()
	if err != nil {
		return nil, err
	}
	for _, ctr := range containers {
		if ctr.ImageID == imageID {
			ctrIDs = append(ctrIDs, ctr.ID)
		}
	}
	return ctrIDs, nil
}

// Removes the containers passed in the array.
func removeStorageContainers(ctrIDs []string, store storage.Store) error {
	for _, ctrID := range ctrIDs {
		if err := store.DeleteContainer(ctrID); err != nil {
			return errors.Wrapf(err, "could not remove container %q", ctrID)
		}
	}
	return nil
}

// Build adds the runtime to the imagebuildah call
func (r *Runtime) Build(ctx context.Context, options imagebuildah.BuildOptions, dockerfiles ...string) error {
	_, _, err := imagebuildah.BuildDockerfiles(ctx, r.store, options, dockerfiles...)
	return err
}

// Import is called as an intermediary to the image library Import
func (r *Runtime) Import(ctx context.Context, source string, reference string, changes []string, history string, quiet bool) (string, error) {
	var (
		writer io.Writer
		err    error
	)

	ic := v1.ImageConfig{}
	if len(changes) > 0 {
		ic, err = util.GetImageConfig(changes)
		if err != nil {
			return "", errors.Wrapf(err, "error adding config changes to image %q", source)
		}
	}

	hist := []v1.History{
		{Comment: history},
	}

	config := v1.Image{
		Config:  ic,
		History: hist,
	}

	writer = nil
	if !quiet {
		writer = os.Stderr
	}

	// if source is a url, download it and save to a temp file
	u, err := url.ParseRequestURI(source)
	if err == nil && u.Scheme != "" {
		file, err := downloadFromURL(source)
		if err != nil {
			return "", err
		}
		defer os.Remove(file)
		source = file
	}
	// if it's stdin, buffer it, too
	if source == "-" {
		file, err := downloadFromFile(os.Stdin)
		if err != nil {
			return "", err
		}
		defer os.Remove(file)
		source = file
	}

	newImage, err := r.imageRuntime.Import(ctx, source, reference, writer, image.SigningOptions{}, config)
	if err != nil {
		return "", err
	}
	return newImage.ID(), nil
}

// donwloadFromURL downloads an image in the format "https:/example.com/myimage.tar"
// and temporarily saves in it /var/tmp/importxyz, which is deleted after the image is imported
func downloadFromURL(source string) (string, error) {
	fmt.Printf("Downloading from %q\n", source)

	outFile, err := ioutil.TempFile("/var/tmp", "import")
	if err != nil {
		return "", errors.Wrap(err, "error creating file")
	}
	defer outFile.Close()

	response, err := http.Get(source)
	if err != nil {
		return "", errors.Wrapf(err, "error downloading %q", source)
	}
	defer response.Body.Close()

	_, err = io.Copy(outFile, response.Body)
	if err != nil {
		return "", errors.Wrapf(err, "error saving %s to %s", source, outFile.Name())
	}

	return outFile.Name(), nil
}

// donwloadFromFile reads all of the content from the reader and temporarily
// saves in it /var/tmp/importxyz, which is deleted after the image is imported
func downloadFromFile(reader *os.File) (string, error) {
	outFile, err := ioutil.TempFile("/var/tmp", "import")
	if err != nil {
		return "", errors.Wrap(err, "error creating file")
	}
	defer outFile.Close()

	logrus.Debugf("saving %s to %s", reader.Name(), outFile.Name())

	_, err = io.Copy(outFile, reader)
	if err != nil {
		return "", errors.Wrapf(err, "error saving %s to %s", reader.Name(), outFile.Name())
	}

	return outFile.Name(), nil
}

// LoadImage loads a container image into local storage
func (r *Runtime) LoadImage(ctx context.Context, name, inputFile string, writer io.Writer, signaturePolicy string) (string, error) {
	var newImages []*image.Image
	src, err := dockerarchive.ParseReference(inputFile) // FIXME? We should add dockerarchive.NewReference()
	if err == nil {
		newImages, err = r.ImageRuntime().LoadFromArchiveReference(ctx, src, signaturePolicy, writer)
	}
	if err != nil {
		// generate full src name with specified image:tag
		src, err := ociarchive.NewReference(inputFile, name) // imageName may be ""
		if err == nil {
			newImages, err = r.ImageRuntime().LoadFromArchiveReference(ctx, src, signaturePolicy, writer)
		}
		if err != nil {
			src, err := directory.NewReference(inputFile)
			if err == nil {
				newImages, err = r.ImageRuntime().LoadFromArchiveReference(ctx, src, signaturePolicy, writer)
			}
			if err != nil {
				return "", errors.Wrapf(err, "error pulling %q", name)
			}
		}
	}
	return getImageNames(newImages), nil
}

func getImageNames(images []*image.Image) string {
	var names string
	for i := range images {
		if i == 0 {
			names = images[i].InputName
		} else {
			names += ", " + images[i].InputName
		}
	}
	return names
}
