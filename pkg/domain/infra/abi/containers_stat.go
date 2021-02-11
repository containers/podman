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
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (ic *ContainerEngine) containerStat(container *libpod.Container, containerMountPoint string, containerPath string) (*entities.ContainerStatReport, string, string, error) {
	// Make sure that "/" copies the *contents* of the mount point and not
	// the directory.
	if containerPath == "/" {
		containerPath += "/."
	}

	// Now resolve the container's path.  It may hit a volume, it may hit a
	// bind mount, it may be relative.
	resolvedRoot, resolvedContainerPath, err := container.ResolvePath(context.Background(), containerMountPoint, containerPath)
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
			statInfoErr = copy.ErrENOENT
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

	containerMountPoint, err := container.Mount()
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := container.Unmount(false); err != nil {
			logrus.Errorf("Error unmounting container: %v", err)
		}
	}()

	statReport, _, _, err := ic.containerStat(container, containerMountPoint, containerPath)
	return statReport, err
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
		return nil, copy.ErrENOENT
	}

	var statErr error
	if stat.Error != "" {
		statErr = errors.New(stat.Error)
	}
	return stat, statErr
}
