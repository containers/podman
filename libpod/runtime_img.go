package libpod

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	buildahDefine "github.com/containers/buildah/define"
	"github.com/containers/buildah/imagebuildah"
	"github.com/containers/common/libimage"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/sirupsen/logrus"
)

// Runtime API

// RemoveContainersForImageCallback returns a callback that can be used in
// `libimage`.  When forcefully removing images, containers using the image
// should be removed as well.  The callback allows for more graceful removal as
// we can use the libpod-internal removal logic.
func (r *Runtime) RemoveContainersForImageCallback(ctx context.Context) libimage.RemoveContainerFunc {
	return func(imageID string) error {
		if !r.valid {
			return define.ErrRuntimeStopped
		}
		ctrs, err := r.state.AllContainers(false)
		if err != nil {
			return err
		}
		for _, ctr := range ctrs {
			if ctr.config.RootfsImageID != imageID {
				continue
			}
			var timeout *uint
			if ctr.config.IsInfra {
				pod, err := r.state.Pod(ctr.config.Pod)
				if err != nil {
					return fmt.Errorf("container %s is in pod %s, but pod cannot be retrieved: %w", ctr.ID(), ctr.config.Pod, err)
				}
				if _, err := r.removePod(ctx, pod, true, true, timeout); err != nil {
					return fmt.Errorf("removing image %s: container %s using image could not be removed: %w", imageID, ctr.ID(), err)
				}
			} else {
				opts := ctrRmOpts{
					Force:   true,
					Timeout: timeout,
				}
				if _, _, err := r.removeContainer(ctx, ctr, opts); err != nil {
					return fmt.Errorf("removing image %s: container %s using image could not be removed: %w", imageID, ctr.ID(), err)
				}
			}
		}

		// Need to handle volumes with the image driver
		vols, err := r.state.AllVolumes()
		if err != nil {
			return err
		}
		for _, vol := range vols {
			if vol.config.Driver != define.VolumeDriverImage || vol.config.StorageImageID != imageID {
				continue
			}
			// Do a force removal of the volume, and all containers
			// using it.
			if err := r.RemoveVolume(ctx, vol, true, nil); err != nil {
				return fmt.Errorf("removing image %s: volume %s backed by image could not be removed: %w", imageID, vol.Name(), err)
			}
		}

		// Note that `libimage` will take care of removing any leftover
		// containers from the storage.
		return nil
	}
}

// IsExternalContainerCallback returns a callback that be used in `libimage` to
// figure out whether a given container is an external one.  A container is
// considered external if it is not present in libpod's database.
func (r *Runtime) IsExternalContainerCallback(_ context.Context) libimage.IsExternalContainerFunc {
	// NOTE: pruning external containers is subject to race conditions
	// (e.g., when a container gets removed). To address this and similar
	// races, pruning had to happen inside c/storage.  Containers has to be
	// labeled with "podman/libpod" along with callbacks similar to
	// libimage.
	return func(idOrName string) (bool, error) {
		_, err := r.LookupContainer(idOrName)
		if err == nil {
			return false, nil
		}
		if errors.Is(err, define.ErrNoSuchCtr) {
			return true, nil
		}
		isVol, err := r.state.ContainerIDIsVolume(idOrName)
		if err == nil && !isVol {
			return true, nil
		}
		return false, nil
	}
}

// newBuildEvent creates a new event based on completion of a built image
func (r *Runtime) newImageBuildCompleteEvent(idOrName string) {
	e := events.NewEvent(events.Build)
	e.Type = events.Image
	e.Name = idOrName
	if err := r.eventer.Write(e); err != nil {
		logrus.Errorf("Unable to write build event: %q", err)
	}
}

// Build adds the runtime to the imagebuildah call
func (r *Runtime) Build(ctx context.Context, options buildahDefine.BuildOptions, dockerfiles ...string) (string, reference.Canonical, error) {
	if options.Runtime == "" {
		options.Runtime = r.GetOCIRuntimePath()
	}
	// share the network interface between podman and buildah
	options.NetworkInterface = r.network
	id, ref, err := imagebuildah.BuildDockerfiles(ctx, r.store, options, dockerfiles...)
	// Write event for build completion
	r.newImageBuildCompleteEvent(id)
	return id, ref, err
}

// DownloadFromFile reads all of the content from the reader and temporarily
// saves in it $TMPDIR/importxyz, which is deleted after the image is imported
func DownloadFromFile(reader *os.File) (string, error) {
	outFile, err := os.CreateTemp(util.Tmpdir(), "import")
	if err != nil {
		return "", fmt.Errorf("creating file: %w", err)
	}
	defer outFile.Close()

	logrus.Debugf("saving %s to %s", reader.Name(), outFile.Name())

	_, err = io.Copy(outFile, reader)
	if err != nil {
		return "", fmt.Errorf("saving %s to %s: %w", reader.Name(), outFile.Name(), err)
	}

	return outFile.Name(), nil
}
