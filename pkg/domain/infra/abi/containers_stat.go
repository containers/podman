package abi

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	buildahCopiah "github.com/containers/buildah/copier"
	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/pkg/copy"
	"github.com/containers/podman/v2/pkg/domain/entities"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (ic *ContainerEngine) containerStat(container *libpod.Container, containerPath string) (*entities.ContainerStatReport, string, string, error) {
	containerMountPoint, err := container.Mount()
	if err != nil {
		return nil, "", "", err
	}

	// Make sure that "/" copies the *contents* of the mount point and not
	// the directory.
	if containerPath == "/" {
		containerPath += "/."
	}

	// Now resolve the container's path.  It may hit a volume, it may hit a
	// bind mount, it may be relative.
	resolvedRoot, resolvedContainerPath, err := resolveContainerPaths(container, containerMountPoint, containerPath)
	if err != nil {
		return nil, "", "", err
	}

	statInfo, statInfoErr := secureStat(resolvedRoot, resolvedContainerPath)
	if statInfoErr != nil {
		// Not all errors from secureStat map to ErrNotExist, so we
		// have to look into the error string.  Turning it into an
		// ENOENT let's the API handlers return the correct status code
		// which is crucial for the remote client.
		if os.IsNotExist(err) || strings.Contains(statInfoErr.Error(), "o such file or directory") {
			statInfoErr = copy.ENOENT
		}
		//  If statInfo is nil, there's nothing we can do anymore.  A
		//  non-nil statInfo may indicate a symlink where we must have
		//  a closer look.
		if statInfo == nil {
			return nil, "", "", statInfoErr
		}
	}

	// Now make sure that the info's LinkTarget is relative to the
	// container's mount.
	var absContainerPath string

	if statInfo.IsSymlink {
		// Evaluated symlinks are always relative to the container's mount point.
		absContainerPath = statInfo.ImmediateTarget
	} else if strings.HasPrefix(resolvedContainerPath, containerMountPoint) {
		// If the path is on the container's mount point, strip it off.
		absContainerPath = strings.TrimPrefix(resolvedContainerPath, containerMountPoint)
		absContainerPath = filepath.Join("/", absContainerPath)
	} else {
		// No symlink and not on the container's mount point, so let's
		// move it back to the original input.  It must have evaluated
		// to a volume or bind mount but we cannot return host paths.
		absContainerPath = containerPath
	}

	// Now we need to make sure to preserve the base path as specified by
	// the user.  The `filepath` packages likes to remove trailing slashes
	// and dots that are crucial to the copy logic.
	absContainerPath = copy.PreserveBasePath(containerPath, absContainerPath)
	resolvedContainerPath = copy.PreserveBasePath(containerPath, resolvedContainerPath)

	info := copy.FileInfo{
		IsDir:      statInfo.IsDir,
		Name:       filepath.Base(absContainerPath),
		Size:       statInfo.Size,
		Mode:       statInfo.Mode,
		ModTime:    statInfo.ModTime,
		LinkTarget: absContainerPath,
	}

	return &entities.ContainerStatReport{FileInfo: info}, resolvedRoot, resolvedContainerPath, statInfoErr
}

func (ic *ContainerEngine) ContainerStat(ctx context.Context, nameOrID string, containerPath string) (*entities.ContainerStatReport, error) {
	container, err := ic.Libpod.LookupContainer(nameOrID)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := container.Unmount(false); err != nil {
			logrus.Errorf("Error unmounting container: %v", err)
		}
	}()

	statReport, _, _, err := ic.containerStat(container, containerPath)
	return statReport, err
}

// resolveContainerPaths resolves the container's mount point and the container
// path as specified by the user.  Both may resolve to paths outside of the
// container's mount point when the container path hits a volume or bind mount.
//
// NOTE: We must take volumes and bind mounts into account as, regrettably, we
// can copy to/from stopped containers.  In that case, the volumes and bind
// mounts are not present.  For running containers, the runtime (e.g., runc or
// crun) takes care of these mounts.  For stopped ones, we need to do quite
// some dance, as done below.
func resolveContainerPaths(container *libpod.Container, mountPoint string, containerPath string) (string, string, error) {
	// Let's first make sure we have a path relative to the mount point.
	pathRelativeToContainerMountPoint := containerPath
	if !filepath.IsAbs(containerPath) {
		// If the containerPath is not absolute, it's relative to the
		// container's working dir.  To be extra careful, let's first
		// join the working dir with "/", and the add the containerPath
		// to it.
		pathRelativeToContainerMountPoint = filepath.Join(filepath.Join("/", container.WorkingDir()), containerPath)
	}
	resolvedPathOnTheContainerMountPoint := filepath.Join(mountPoint, pathRelativeToContainerMountPoint)
	pathRelativeToContainerMountPoint = strings.TrimPrefix(pathRelativeToContainerMountPoint, mountPoint)
	pathRelativeToContainerMountPoint = filepath.Join("/", pathRelativeToContainerMountPoint)

	// Now we have an "absolute container Path" but not yet resolved on the
	// host (e.g., "/foo/bar/file.txt").  As mentioned above, we need to
	// check if "/foo/bar/file.txt" is on a volume or bind mount.  To do
	// that, we need to walk *down* the paths to the root.  Assuming
	// volume-1 is mounted to "/foo" and volume-2 is mounted to "/foo/bar",
	// we must select "/foo/bar".  Once selected, we need to rebase the
	// remainder (i.e, "/file.txt") on the volume's mount point on the
	// host.  Same applies to bind mounts.

	searchPath := pathRelativeToContainerMountPoint
	for {
		volume, err := findVolume(container, searchPath)
		if err != nil {
			return "", "", err
		}
		if volume != nil {
			logrus.Debugf("Container path %q resolved to volume %q on path %q", containerPath, volume.Name(), searchPath)
			// We found a matching volume for searchPath.  We now
			// need to first find the relative path of our input
			// path to the searchPath, and then join it with the
			// volume's mount point.
			pathRelativeToVolume := strings.TrimPrefix(pathRelativeToContainerMountPoint, searchPath)
			absolutePathOnTheVolumeMount, err := securejoin.SecureJoin(volume.MountPoint(), pathRelativeToVolume)
			if err != nil {
				return "", "", err
			}
			return volume.MountPoint(), absolutePathOnTheVolumeMount, nil
		}

		if mount := findBindMount(container, searchPath); mount != nil {
			logrus.Debugf("Container path %q resolved to bind mount %q:%q on path %q", containerPath, mount.Source, mount.Destination, searchPath)
			// We found a matching bind mount for searchPath.  We
			// now need to first find the relative path of our
			// input path to the searchPath, and then join it with
			// the source of the bind mount.
			pathRelativeToBindMount := strings.TrimPrefix(pathRelativeToContainerMountPoint, searchPath)
			absolutePathOnTheBindMount, err := securejoin.SecureJoin(mount.Source, pathRelativeToBindMount)
			if err != nil {
				return "", "", err
			}
			return mount.Source, absolutePathOnTheBindMount, nil

		}

		if searchPath == "/" {
			// Cannot go beyond "/", so we're done.
			break
		}

		// Walk *down* the path (e.g., "/foo/bar/x" -> "/foo/bar").
		searchPath = filepath.Dir(searchPath)
	}

	// No volume, no bind mount but just a normal path on the container.
	return mountPoint, resolvedPathOnTheContainerMountPoint, nil
}

// findVolume checks if the specified container path matches a volume inside
// the container.  It returns a matching volume or nil.
func findVolume(c *libpod.Container, containerPath string) (*libpod.Volume, error) {
	runtime := c.Runtime()
	cleanedContainerPath := filepath.Clean(containerPath)
	for _, vol := range c.Config().NamedVolumes {
		if cleanedContainerPath == filepath.Clean(vol.Dest) {
			return runtime.GetVolume(vol.Name)
		}
	}
	return nil, nil
}

// findBindMount checks if the specified container path matches a bind mount
// inside the container.  It returns a matching mount or nil.
func findBindMount(c *libpod.Container, containerPath string) *specs.Mount {
	cleanedPath := filepath.Clean(containerPath)
	for _, m := range c.Config().Spec.Mounts {
		if m.Type != "bind" {
			continue
		}
		if cleanedPath == filepath.Clean(m.Destination) {
			mount := m
			return &mount
		}
	}
	return nil
}

// secureStat extracts file info for path in a chroot'ed environment in root.
func secureStat(root string, path string) (*buildahCopiah.StatForItem, error) {
	var glob string
	var err error

	// If root and path are equal, then dir must be empty and the glob must
	// be ".".
	if filepath.Clean(root) == filepath.Clean(path) {
		glob = "."
	} else {
		glob, err = filepath.Rel(root, path)
		if err != nil {
			return nil, err
		}
	}

	globStats, err := buildahCopiah.Stat(root, "", buildahCopiah.StatOptions{}, []string{glob})
	if err != nil {
		return nil, err
	}

	if len(globStats) != 1 {
		return nil, errors.Errorf("internal error: secureStat: expected 1 item but got %d", len(globStats))
	}

	stat, exists := globStats[0].Results[glob] // only one glob passed, so that's okay
	if !exists {
		return nil, copy.ENOENT
	}

	var statErr error
	if stat.Error != "" {
		statErr = errors.New(stat.Error)
	}
	return stat, statErr
}
