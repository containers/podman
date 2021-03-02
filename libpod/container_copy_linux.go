// +build linux

package libpod

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	buildahCopiah "github.com/containers/buildah/copier"
	"github.com/containers/buildah/pkg/chrootuser"
	"github.com/containers/buildah/util"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/idtools"
	"github.com/docker/docker/pkg/archive"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func (c *Container) copyFromArchive(ctx context.Context, path string, reader io.Reader) (func() error, error) {
	var (
		mountPoint   string
		resolvedRoot string
		resolvedPath string
		unmount      func()
		err          error
	)

	// Make sure that "/" copies the *contents* of the mount point and not
	// the directory.
	if path == "/" {
		path = "/."
	}

	// Optimization: only mount if the container is not already.
	if c.state.Mounted {
		mountPoint = c.state.Mountpoint
		unmount = func() {}
	} else {
		// NOTE: make sure to unmount in error paths.
		mountPoint, err = c.mount()
		if err != nil {
			return nil, err
		}
		unmount = func() { c.unmount(false) }
	}

	if c.state.State == define.ContainerStateRunning {
		resolvedRoot = "/"
		resolvedPath = c.pathAbs(path)
	} else {
		resolvedRoot, resolvedPath, err = c.resolvePath(mountPoint, path)
		if err != nil {
			unmount()
			return nil, err
		}
	}

	decompressed, err := archive.DecompressStream(reader)
	if err != nil {
		unmount()
		return nil, err
	}

	idMappings, idPair, err := getIDMappingsAndPair(c, mountPoint)
	if err != nil {
		decompressed.Close()
		unmount()
		return nil, err
	}

	logrus.Debugf("Container copy *to* %q (resolved: %q) on container %q (ID: %s)", path, resolvedPath, c.Name(), c.ID())

	return func() error {
		defer unmount()
		defer decompressed.Close()
		putOptions := buildahCopiah.PutOptions{
			UIDMap:     idMappings.UIDMap,
			GIDMap:     idMappings.GIDMap,
			ChownDirs:  idPair,
			ChownFiles: idPair,
		}

		return c.joinMountAndExec(ctx,
			func() error {
				return buildahCopiah.Put(resolvedRoot, resolvedPath, putOptions, decompressed)
			},
		)
	}, nil
}

func (c *Container) copyToArchive(ctx context.Context, path string, writer io.Writer) (func() error, error) {
	var (
		mountPoint string
		unmount    func()
		err        error
	)

	// Optimization: only mount if the container is not already.
	if c.state.Mounted {
		mountPoint = c.state.Mountpoint
		unmount = func() {}
	} else {
		// NOTE: make sure to unmount in error paths.
		mountPoint, err = c.mount()
		if err != nil {
			return nil, err
		}
		unmount = func() { c.unmount(false) }
	}

	statInfo, resolvedRoot, resolvedPath, err := c.stat(ctx, mountPoint, path)
	if err != nil {
		unmount()
		return nil, err
	}

	idMappings, idPair, err := getIDMappingsAndPair(c, mountPoint)
	if err != nil {
		unmount()
		return nil, err
	}

	logrus.Debugf("Container copy *from* %q (resolved: %q) on container %q (ID: %s)", path, resolvedPath, c.Name(), c.ID())

	return func() error {
		defer unmount()
		getOptions := buildahCopiah.GetOptions{
			// Unless the specified points to ".", we want to copy the base directory.
			KeepDirectoryNames: statInfo.IsDir && filepath.Base(path) != ".",
			UIDMap:             idMappings.UIDMap,
			GIDMap:             idMappings.GIDMap,
			ChownDirs:          idPair,
			ChownFiles:         idPair,
			Excludes:           []string{"dev", "proc", "sys"},
		}
		return c.joinMountAndExec(ctx,
			func() error {
				return buildahCopiah.Get(resolvedRoot, "", getOptions, []string{resolvedPath}, writer)
			},
		)
	}, nil
}

// getIDMappingsAndPair returns the ID mappings for the container and the host
// ID pair.
func getIDMappingsAndPair(container *Container, containerMount string) (*storage.IDMappingOptions, *idtools.IDPair, error) {
	user, err := getContainerUser(container, containerMount)
	if err != nil {
		return nil, nil, err
	}

	idMappingOpts, err := container.IDMappings()
	if err != nil {
		return nil, nil, err
	}

	hostUID, hostGID, err := util.GetHostIDs(idtoolsToRuntimeSpec(idMappingOpts.UIDMap), idtoolsToRuntimeSpec(idMappingOpts.GIDMap), user.UID, user.GID)
	if err != nil {
		return nil, nil, err
	}

	idPair := idtools.IDPair{UID: int(hostUID), GID: int(hostGID)}
	return &idMappingOpts, &idPair, nil
}

// getContainerUser returns the specs.User of the container.
func getContainerUser(container *Container, mountPoint string) (specs.User, error) {
	userspec := container.Config().User

	uid, gid, _, err := chrootuser.GetUser(mountPoint, userspec)
	u := specs.User{
		UID:      uid,
		GID:      gid,
		Username: userspec,
	}

	if !strings.Contains(userspec, ":") {
		groups, err2 := chrootuser.GetAdditionalGroupsForUser(mountPoint, uint64(u.UID))
		if err2 != nil {
			if errors.Cause(err2) != chrootuser.ErrNoSuchUser && err == nil {
				err = err2
			}
		} else {
			u.AdditionalGids = groups
		}
	}

	return u, err
}

// idtoolsToRuntimeSpec converts idtools ID mapping to the one of the runtime spec.
func idtoolsToRuntimeSpec(idMaps []idtools.IDMap) (convertedIDMap []specs.LinuxIDMapping) {
	for _, idmap := range idMaps {
		tempIDMap := specs.LinuxIDMapping{
			ContainerID: uint32(idmap.ContainerID),
			HostID:      uint32(idmap.HostID),
			Size:        uint32(idmap.Size),
		}
		convertedIDMap = append(convertedIDMap, tempIDMap)
	}
	return convertedIDMap
}

// joinMountAndExec executes the specified function `f` inside the container's
// mount and PID namespace.  That allows for having the exact view on the
// container's file system.
//
// Note, if the container is not running `f()` will be executed as is.
func (c *Container) joinMountAndExec(ctx context.Context, f func() error) error {
	if c.state.State != define.ContainerStateRunning {
		return f()
	}

	// Container's running, so we need to execute `f()` inside its mount NS.
	errChan := make(chan error)
	go func() {
		runtime.LockOSThread()

		// Join the mount and PID NS of the container.
		getFD := func(ns LinuxNS) (*os.File, error) {
			nsPath, err := c.namespacePath(ns)
			if err != nil {
				return nil, err
			}
			return os.Open(nsPath)
		}

		mountFD, err := getFD(MountNS)
		if err != nil {
			errChan <- err
			return
		}
		defer mountFD.Close()

		pidFD, err := getFD(PIDNS)
		if err != nil {
			errChan <- err
			return
		}
		defer pidFD.Close()
		if err := unix.Unshare(unix.CLONE_NEWNS); err != nil {
			errChan <- err
			return
		}
		if err := unix.Setns(int(pidFD.Fd()), unix.CLONE_NEWPID); err != nil {
			errChan <- err
			return
		}

		if err := unix.Setns(int(mountFD.Fd()), unix.CLONE_NEWNS); err != nil {
			errChan <- err
			return
		}

		// Last but not least, execute the workload.
		errChan <- f()
	}()
	return <-errChan
}
