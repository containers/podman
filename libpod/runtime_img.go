package libpod

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/containers/buildah/imagebuildah"
	"github.com/containers/image/v5/directory"
	"github.com/containers/image/v5/docker/reference"
	ociarchive "github.com/containers/image/v5/oci/archive"
	"github.com/containers/image/v5/oci/layout"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/libpod/events"
	"github.com/containers/podman/v2/libpod/image"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	dockerarchive "github.com/containers/image/v5/docker/archive"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// Runtime API

// RemoveImage deletes an image from local storage
// Images being used by running containers can only be removed if force=true
func (r *Runtime) RemoveImage(ctx context.Context, img *image.Image, force bool) (*image.ImageDeleteResponse, error) {
	response := image.ImageDeleteResponse{}
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.valid {
		return nil, define.ErrRuntimeStopped
	}

	// Get all containers, filter to only those using the image, and remove those containers
	ctrs, err := r.state.AllContainers()
	if err != nil {
		return nil, err
	}
	imageCtrs := []*Container{}
	for _, ctr := range ctrs {
		if ctr.config.RootfsImageID == img.ID() {
			imageCtrs = append(imageCtrs, ctr)
		}
	}
	if len(imageCtrs) > 0 && (len(img.Names()) <= 1 || (force && img.InputIsID())) {
		if force {
			for _, ctr := range imageCtrs {
				if err := r.removeContainer(ctx, ctr, true, false, false); err != nil {
					return nil, errors.Wrapf(err, "error removing image %s: container %s using image could not be removed", img.ID(), ctr.ID())
				}
			}
		} else {
			return nil, errors.Wrapf(define.ErrImageInUse, "could not remove image %s as it is being used by %d containers", img.ID(), len(imageCtrs))
		}
	}

	hasChildren, err := img.IsParent(ctx)
	if err != nil {
		return nil, err
	}

	if (len(img.Names()) > 1 && !img.InputIsID()) || hasChildren {
		// If the image has multiple reponames, we do not technically delete
		// the image. we figure out which repotag the user is trying to refer
		// to and untag it.
		repoName, err := img.MatchRepoTag(img.InputName)
		if hasChildren && errors.Cause(err) == image.ErrRepoTagNotFound {
			return nil, errors.Wrapf(define.ErrImageInUse,
				"unable to delete %q (cannot be forced) - image has dependent child images", img.ID())
		}
		if err != nil {
			return nil, err
		}
		if err := img.UntagImage(repoName); err != nil {
			return nil, err
		}
		response.Untagged = append(response.Untagged, repoName)
		return &response, nil
	} else if len(img.Names()) > 1 && img.InputIsID() && !force {
		// If the user requests to delete an image by ID and the image has multiple
		// reponames and no force is applied, we error out.
		return nil, errors.Wrapf(define.ErrImageInUse,
			"unable to delete %s (must force) - image is referred to in multiple tags", img.ID())
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
	response.Untagged = append(response.Untagged, img.Names()...)
	response.Deleted = img.ID()
	return &response, err
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
		if _, err := store.Unmount(ctrID, true); err != nil {
			return errors.Wrapf(err, "could not unmount container %q to remove it", ctrID)
		}

		if err := store.DeleteContainer(ctrID); err != nil {
			return errors.Wrapf(err, "could not remove container %q", ctrID)
		}
	}
	return nil
}

// newBuildEvent creates a new event based on completion of a built image
func (r *Runtime) newImageBuildCompleteEvent(idOrName string) {
	e := events.NewEvent(events.Build)
	e.Type = events.Image
	e.Name = idOrName
	if err := r.eventer.Write(e); err != nil {
		logrus.Errorf("unable to write build event: %q", err)
	}
}

// Build adds the runtime to the imagebuildah call
func (r *Runtime) Build(ctx context.Context, options imagebuildah.BuildOptions, dockerfiles ...string) (string, reference.Canonical, error) {
	id, ref, err := imagebuildah.BuildDockerfiles(ctx, r.store, options, dockerfiles...)
	// Write event for build completion
	r.newImageBuildCompleteEvent(id)
	return id, ref, err
}

// Import is called as an intermediary to the image library Import
func (r *Runtime) Import(ctx context.Context, source, reference, signaturePolicyPath string, changes []string, history string, quiet bool) (string, error) {
	var (
		writer io.Writer
		err    error
	)

	ic := v1.ImageConfig{}
	if len(changes) > 0 {
		config, err := util.GetImageConfig(changes)
		if err != nil {
			return "", errors.Wrapf(err, "error adding config changes to image %q", source)
		}
		ic = config.ImageConfig
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
		file, err := DownloadFromFile(os.Stdin)
		if err != nil {
			return "", err
		}
		defer os.Remove(file)
		source = file
	}

	r.imageRuntime.SignaturePolicyPath = signaturePolicyPath
	newImage, err := r.imageRuntime.Import(ctx, source, reference, writer, image.SigningOptions{}, config)
	if err != nil {
		return "", err
	}
	return newImage.ID(), nil
}

// donwloadFromURL downloads an image in the format "https:/example.com/myimage.tar"
// and temporarily saves in it $TMPDIR/importxyz, which is deleted after the image is imported
func downloadFromURL(source string) (string, error) {
	fmt.Printf("Downloading from %q\n", source)

	outFile, err := ioutil.TempFile(util.Tmpdir(), "import")
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

// DownloadFromFile reads all of the content from the reader and temporarily
// saves in it $TMPDIR/importxyz, which is deleted after the image is imported
func DownloadFromFile(reader *os.File) (string, error) {
	outFile, err := ioutil.TempFile(util.Tmpdir(), "import")
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
	var (
		newImages []*image.Image
		err       error
		src       types.ImageReference
	)

	if name == "" {
		newImages, err = r.ImageRuntime().LoadAllImagesFromDockerArchive(ctx, inputFile, signaturePolicy, writer)
		if err == nil {
			return getImageNames(newImages), nil
		}
	}

	for _, referenceFn := range []func() (types.ImageReference, error){
		func() (types.ImageReference, error) {
			return dockerarchive.ParseReference(inputFile)
		},
		func() (types.ImageReference, error) {
			return ociarchive.NewReference(inputFile, name) // name may be ""
		},
		func() (types.ImageReference, error) {
			// prepend "localhost/" to support local image saved with this semantics
			if !strings.Contains(name, "/") {
				return ociarchive.NewReference(inputFile, fmt.Sprintf("%s/%s", image.DefaultLocalRegistry, name))
			}
			return nil, nil
		},
		func() (types.ImageReference, error) {
			return directory.NewReference(inputFile)
		},
		func() (types.ImageReference, error) {
			return layout.NewReference(inputFile, name)
		},
		func() (types.ImageReference, error) {
			// prepend "localhost/" to support local image saved with this semantics
			if !strings.Contains(name, "/") {
				return layout.NewReference(inputFile, fmt.Sprintf("%s/%s", image.DefaultLocalRegistry, name))
			}
			return nil, nil
		},
	} {
		src, err = referenceFn()
		if err == nil && src != nil {
			if newImages, err = r.ImageRuntime().LoadFromArchiveReference(ctx, src, signaturePolicy, writer); err == nil {
				return getImageNames(newImages), nil
			}
		}
	}
	return "", errors.Wrapf(err, "error pulling %q", name)
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
