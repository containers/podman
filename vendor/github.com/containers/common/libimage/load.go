package libimage

import (
	"context"
	"fmt"
	"os"
	"time"

	dirTransport "github.com/containers/image/v5/directory"
	dockerArchiveTransport "github.com/containers/image/v5/docker/archive"
	ociArchiveTransport "github.com/containers/image/v5/oci/archive"
	ociTransport "github.com/containers/image/v5/oci/layout"
	"github.com/containers/image/v5/types"
	"github.com/sirupsen/logrus"
)

type LoadOptions struct {
	CopyOptions
}

// Load loads one or more images (depending on the transport) from the
// specified path.  The path may point to an image the following transports:
// oci, oci-archive, dir, docker-archive.
func (r *Runtime) Load(ctx context.Context, path string, options *LoadOptions) ([]string, error) {
	logrus.Debugf("Loading image from %q", path)

	if r.eventChannel != nil {
		defer r.writeEvent(&Event{ID: "", Name: path, Time: time.Now(), Type: EventTypeImageLoad})
	}

	if options == nil {
		options = &LoadOptions{}
	}

	// we have 4 functions, so a maximum of 4 errors
	loadErrors := make([]error, 0, 4)
	for _, f := range []func() ([]string, string, error){
		// OCI
		func() ([]string, string, error) {
			logrus.Debugf("-> Attempting to load %q as an OCI directory", path)
			ref, err := ociTransport.NewReference(path, "")
			if err != nil {
				return nil, ociTransport.Transport.Name(), err
			}
			images, err := r.copyFromDefault(ctx, ref, &options.CopyOptions)
			return images, ociTransport.Transport.Name(), err
		},

		// OCI-ARCHIVE
		func() ([]string, string, error) {
			logrus.Debugf("-> Attempting to load %q as an OCI archive", path)
			ref, err := ociArchiveTransport.NewReference(path, "")
			if err != nil {
				return nil, ociArchiveTransport.Transport.Name(), err
			}
			images, err := r.copyFromDefault(ctx, ref, &options.CopyOptions)
			return images, ociArchiveTransport.Transport.Name(), err
		},

		// DOCKER-ARCHIVE
		func() ([]string, string, error) {
			logrus.Debugf("-> Attempting to load %q as a Docker archive", path)
			ref, err := dockerArchiveTransport.ParseReference(path)
			if err != nil {
				return nil, dockerArchiveTransport.Transport.Name(), err
			}
			images, err := r.loadMultiImageDockerArchive(ctx, ref, &options.CopyOptions)
			return images, dockerArchiveTransport.Transport.Name(), err
		},

		// DIR
		func() ([]string, string, error) {
			logrus.Debugf("-> Attempting to load %q as a Docker dir", path)
			ref, err := dirTransport.NewReference(path)
			if err != nil {
				return nil, dirTransport.Transport.Name(), err
			}
			images, err := r.copyFromDefault(ctx, ref, &options.CopyOptions)
			return images, dirTransport.Transport.Name(), err
		},
	} {
		loadedImages, transportName, err := f()
		if err == nil {
			return loadedImages, nil
		}
		logrus.Debugf("Error loading %s (%s): %v", path, transportName, err)
		loadErrors = append(loadErrors, fmt.Errorf("%s: %v", transportName, err))
	}

	// Give a decent error message if nothing above worked.
	// we want the colon here for the multiline error
	//nolint:revive
	loadError := fmt.Errorf("payload does not match any of the supported image formats:")
	for _, err := range loadErrors {
		loadError = fmt.Errorf("%v\n * %v", loadError, err)
	}

	return nil, loadError
}

// loadMultiImageDockerArchive loads the docker archive specified by ref.  In
// case the path@reference notation was used, only the specified image will be
// loaded.  Otherwise, all images will be loaded.
func (r *Runtime) loadMultiImageDockerArchive(ctx context.Context, ref types.ImageReference, options *CopyOptions) ([]string, error) {
	// If we cannot stat the path, it either does not exist OR the correct
	// syntax to reference an image within the archive was used, so we
	// should.
	path := ref.StringWithinTransport()
	if _, err := os.Stat(path); err != nil {
		return r.copyFromDockerArchive(ctx, ref, options)
	}

	reader, err := dockerArchiveTransport.NewReader(r.systemContextCopy(), path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := reader.Close(); err != nil {
			logrus.Errorf("Closing reader of docker archive: %v", err)
		}
	}()

	refLists, err := reader.List()
	if err != nil {
		return nil, err
	}

	var copiedImages []string
	for _, list := range refLists {
		for _, listRef := range list {
			names, err := r.copyFromDockerArchiveReaderReference(ctx, reader, listRef, options)
			if err != nil {
				return nil, err
			}
			copiedImages = append(copiedImages, names...)
		}
	}

	return copiedImages, nil
}
