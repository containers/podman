package libpod

import (
	"context"
	"io"
	"io/ioutil"
	"os"

	buildahDefine "github.com/containers/buildah/define"
	"github.com/containers/buildah/imagebuildah"
	"github.com/containers/common/libimage"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/libpod/events"
	"github.com/containers/podman/v3/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Runtime API

// RemoveContainersForImageCallback returns a callback that can be used in
// `libimage`.  When forcefully removing images, containers using the image
// should be removed as well.  The callback allows for more graceful removal as
// we can use the libpod-internal removal logic.
func (r *Runtime) RemoveContainersForImageCallback(ctx context.Context) libimage.RemoveContainerFunc {
	return func(imageID string) error {
		r.lock.Lock()
		defer r.lock.Unlock()

		if !r.valid {
			return define.ErrRuntimeStopped
		}
		ctrs, err := r.state.AllContainers()
		if err != nil {
			return err
		}
		for _, ctr := range ctrs {
			if ctr.config.RootfsImageID == imageID {
				if err := r.removeContainer(ctx, ctr, true, false, false); err != nil {
					return errors.Wrapf(err, "error removing image %s: container %s using image could not be removed", imageID, ctr.ID())
				}
			}
		}
		// Note that `libimage` will take care of removing any leftover
		// containers from the storage.
		return nil
	}
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
func (r *Runtime) Build(ctx context.Context, options buildahDefine.BuildOptions, dockerfiles ...string) (string, reference.Canonical, error) {
	if options.Runtime == "" {
		options.Runtime = r.GetOCIRuntimePath()
	}
	id, ref, err := imagebuildah.BuildDockerfiles(ctx, r.store, options, dockerfiles...)
	// Write event for build completion
	r.newImageBuildCompleteEvent(id)
	return id, ref, err
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
