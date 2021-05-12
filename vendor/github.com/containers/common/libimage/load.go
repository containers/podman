package libimage

import (
	"context"
	"errors"
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
		r.writeEvent(&Event{ID: "", Name: path, Time: time.Now(), Type: EventTypeImageLoad})
	}

	var (
		loadedImages []string
		loadError    error
	)

	if options == nil {
		options = &LoadOptions{}
	}

	for _, f := range []func() ([]string, error){
		// OCI
		func() ([]string, error) {
			logrus.Debugf("-> Attempting to load %q as an OCI directory", path)
			ref, err := ociTransport.NewReference(path, "")
			if err != nil {
				return nil, err
			}
			return r.copyFromDefault(ctx, ref, &options.CopyOptions)
		},

		// OCI-ARCHIVE
		func() ([]string, error) {
			logrus.Debugf("-> Attempting to load %q as an OCI archive", path)
			ref, err := ociArchiveTransport.NewReference(path, "")
			if err != nil {
				return nil, err
			}
			return r.copyFromDefault(ctx, ref, &options.CopyOptions)
		},

		// DIR
		func() ([]string, error) {
			logrus.Debugf("-> Attempting to load %q as a Docker dir", path)
			ref, err := dirTransport.NewReference(path)
			if err != nil {
				return nil, err
			}
			return r.copyFromDefault(ctx, ref, &options.CopyOptions)
		},

		// DOCKER-ARCHIVE
		func() ([]string, error) {
			logrus.Debugf("-> Attempting to load %q as a Docker archive", path)
			ref, err := dockerArchiveTransport.ParseReference(path)
			if err != nil {
				return nil, err
			}
			return r.loadMultiImageDockerArchive(ctx, ref, &options.CopyOptions)
		},

		// Give a decent error message if nothing above worked.
		func() ([]string, error) {
			return nil, errors.New("payload does not match any of the supported image formats (oci, oci-archive, dir, docker-archive)")
		},
	} {
		loadedImages, loadError = f()
		if loadError == nil {
			return loadedImages, loadError
		}
		logrus.Debugf("Error loading %s: %v", path, loadError)
	}

	return nil, loadError
}

// loadMultiImageDockerArchive loads the docker archive specified by ref.  In
// case the path@reference notation was used, only the specifiec image will be
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
