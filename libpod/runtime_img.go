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
	specs "github.com/opencontainers/runtime-spec/specs-go"
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
			if ctr.config.RootfsImageID != imageID {
				continue
			}
			var timeout *uint
			if ctr.config.IsInfra {
				pod, err := r.state.Pod(ctr.config.Pod)
				if err != nil {
					return errors.Wrapf(err, "container %s is in pod %s, but pod cannot be retrieved", ctr.ID(), pod.ID())
				}
				if err := r.removePod(ctx, pod, true, true, timeout); err != nil {
					return errors.Wrapf(err, "removing image %s: container %s using image could not be removed", imageID, ctr.ID())
				}
			} else {
				if err := r.removeContainer(ctx, ctr, true, false, false, timeout); err != nil {
					return errors.Wrapf(err, "removing image %s: container %s using image could not be removed", imageID, ctr.ID())
				}
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
	// labelled with "podman/libpod" along with callbacks similar to
	// libimage.
	return func(idOrName string) (bool, error) {
		_, err := r.LookupContainer(idOrName)
		if err == nil {
			return false, nil
		}
		if errors.Is(err, define.ErrNoSuchCtr) {
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
	netnsFound := false
	for _, ns := range options.NamespaceOptions {
		if ns.Name == string(specs.NetworkNamespace) {
			netnsFound = true
			break
		}
	}
	if !netnsFound {
		options.NamespaceOptions = append(options.NamespaceOptions, buildahDefine.NamespaceOption{
			Name: string(specs.NetworkNamespace),
			Host: true,
		})
	}
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
