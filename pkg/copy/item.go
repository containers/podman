package copy

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	buildahCopiah "github.com/containers/buildah/copier"
	"github.com/containers/buildah/pkg/chrootuser"
	"github.com/containers/buildah/util"
	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/cgroups"
	"github.com/containers/podman/v2/pkg/rootless"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/idtools"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ********************************* NOTE *************************************
//
// Most security bugs are caused by attackers playing around with symlinks
// trying to escape from the container onto the host and/or trick into data
// corruption on the host.  Hence, file operations on containers (including
// *stat) should always be handled by `github.com/containers/buildah/copier`
// which makes sure to evaluate files in a chroot'ed environment.
//
// Please make sure to add verbose comments when changing code to make the
// lives of future readers easier.
//
// ****************************************************************************

var (
	_stdin  = os.Stdin.Name()
	_stdout = os.Stdout.Name()
)

// CopyItem is the source or destination of a copy operation.  Use the
// CopyItemFrom* functions to create one for the specific source/destination
// item.
type CopyItem struct {
	// The original path provided by the caller.  Useful in error messages.
	original string
	// The resolved path on the host or container.  Maybe altered at
	// multiple stages when copying.
	resolved string
	// The root for copying data in a chroot'ed environment.
	root string

	// IDPair of the resolved path.
	idPair *idtools.IDPair
	// Storage ID mappings.
	idMappings *storage.IDMappingOptions

	// Internal FileInfo.  We really don't want users to mess with a
	// CopyItem but only plug and play with it.
	info FileInfo
	// Error when creating the upper FileInfo.  Some errors are non-fatal,
	// for instance, when a destination *base* path does not exist.
	statError error

	writer io.Writer
	reader io.Reader

	// Needed to clean up resources (e.g., unmount a container).
	cleanUpFuncs []deferFunc
}

// deferFunc allows for returning functions that must be deferred at call sites.
type deferFunc func()

// FileInfo describes a file or directory and is returned by
// (*CopyItem).Stat().
type FileInfo struct {
	Name       string      `json:"name"`
	Size       int64       `json:"size"`
	Mode       os.FileMode `json:"mode"`
	ModTime    time.Time   `json:"mtime"`
	IsDir      bool        `json:"isDir"`
	IsStream   bool        `json:"isStream"`
	LinkTarget string      `json:"linkTarget"`
}

// Stat returns the FileInfo.
func (item *CopyItem) Stat() (*FileInfo, error) {
	return &item.info, item.statError
}

// CleanUp releases resources such as the container mounts.  It *must* be
// called even in case of errors.
func (item *CopyItem) CleanUp() {
	for _, f := range item.cleanUpFuncs {
		f()
	}
}

// CopyItemForWriter returns a CopyItem for the specified io.WriteCloser.  Note
// that the returned item can only act as a copy destination.
func CopyItemForWriter(writer io.Writer) (item CopyItem, _ error) {
	item.writer = writer
	item.info.IsStream = true
	return item, nil
}

// CopyItemForReader returns a CopyItem for the specified io.ReaderCloser.  Note
// that the returned item can only act as a copy source.
//
// Note that the specified reader will be auto-decompressed if needed.
func CopyItemForReader(reader io.Reader) (item CopyItem, _ error) {
	item.info.IsStream = true
	decompressed, err := archive.DecompressStream(reader)
	if err != nil {
		return item, err
	}
	item.reader = decompressed
	item.cleanUpFuncs = append(item.cleanUpFuncs, func() {
		if err := decompressed.Close(); err != nil {
			logrus.Errorf("Error closing decompressed reader of copy item: %v", err)
		}
	})
	return item, nil
}

// CopyItemForHost creates a CopyItem for the specified host path.  It's a
// destination by default.  Use isSource to set it as a destination.
//
// Note that callers *must* call (CopyItem).CleanUp(), even in case of errors.
func CopyItemForHost(hostPath string, isSource bool) (item CopyItem, _ error) {
	if hostPath == "-" {
		if isSource {
			hostPath = _stdin
		} else {
			hostPath = _stdout
		}
	}

	if hostPath == _stdin {
		return CopyItemForReader(os.Stdin)
	}

	if hostPath == _stdout {
		return CopyItemForWriter(os.Stdout)
	}

	// Now do the dance for the host data.
	resolvedHostPath, err := filepath.Abs(hostPath)
	if err != nil {
		return item, err
	}

	resolvedHostPath = preserveBasePath(hostPath, resolvedHostPath)
	item.original = hostPath
	item.resolved = resolvedHostPath
	item.root = "/"

	statInfo, statError := os.Stat(resolvedHostPath)
	item.statError = statError

	// It exists, we're done.
	if statError == nil {
		item.info.Name = statInfo.Name()
		item.info.Size = statInfo.Size()
		item.info.Mode = statInfo.Mode()
		item.info.ModTime = statInfo.ModTime()
		item.info.IsDir = statInfo.IsDir()
		item.info.LinkTarget = resolvedHostPath
		return item, nil
	}

	// The source must exist, but let's try to give some human-friendly
	// errors.
	if isSource {
		if os.IsNotExist(item.statError) {
			return item, errors.Wrapf(os.ErrNotExist, "%q could not be found on the host", hostPath)
		}
		return item, item.statError // could be a permission error
	}

	// If we're a destination, we need to make sure that the parent
	// directory exists.
	parent := filepath.Dir(resolvedHostPath)
	if _, err := os.Stat(parent); err != nil {
		if os.IsNotExist(err) {
			return item, errors.Wrapf(os.ErrNotExist, "%q could not be found on the host", parent)
		}
		return item, err
	}

	return item, nil
}

// CopyItemForContainer creates a CopyItem for the specified path on the
// container.  It's a destination by default.  Use isSource to set it as a
// destination.  Note that the container path may resolve to a path outside of
// the container's mount point if the path hits a volume or mount on the
// container.
//
// Note that callers *must* call (CopyItem).CleanUp(), even in case of errors.
func CopyItemForContainer(container *libpod.Container, containerPath string, pause bool, isSource bool) (item CopyItem, _ error) {
	// Mount and pause the container.
	containerMountPoint, err := item.mountAndPauseContainer(container, pause)
	if err != nil {
		return item, err
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
		return item, err
	}
	resolvedContainerPath = preserveBasePath(containerPath, resolvedContainerPath)

	idMappings, idPair, err := getIDMappingsAndPair(container, containerMountPoint)
	if err != nil {
		return item, err
	}

	item.original = containerPath
	item.resolved = resolvedContainerPath
	item.root = resolvedRoot
	item.idMappings = idMappings
	item.idPair = idPair

	statInfo, statError := secureStat(resolvedRoot, resolvedContainerPath)
	item.statError = statError

	// It exists, we're done.
	if statError == nil {
		item.info.IsDir = statInfo.IsDir
		item.info.Name = filepath.Base(statInfo.Name)
		item.info.Size = statInfo.Size
		item.info.Mode = statInfo.Mode
		item.info.ModTime = statInfo.ModTime
		item.info.IsDir = statInfo.IsDir
		item.info.LinkTarget = resolvedContainerPath
		return item, nil
	}

	// The source must exist, but let's try to give some human-friendly
	// errors.
	if isSource {
		if os.IsNotExist(statError) {
			return item, errors.Wrapf(os.ErrNotExist, "%q could not be found on container %s (resolved to %q)", containerPath, container.ID(), resolvedContainerPath)
		}
		return item, item.statError // could be a permission error
	}

	// If we're a destination, we need to make sure that the parent
	// directory exists.
	parent := filepath.Dir(resolvedContainerPath)
	if _, err := secureStat(resolvedRoot, parent); err != nil {
		if os.IsNotExist(err) {
			return item, errors.Wrapf(os.ErrNotExist, "%q could not be found on container %s (resolved to %q)", containerPath, container.ID(), resolvedContainerPath)
		}
		return item, err
	}

	return item, nil
}

// putOptions returns PUT options for buildah's copier package.
func (item *CopyItem) putOptions() buildahCopiah.PutOptions {
	options := buildahCopiah.PutOptions{}
	if item.idMappings != nil {
		options.UIDMap = item.idMappings.UIDMap
		options.GIDMap = item.idMappings.GIDMap
	}
	if item.idPair != nil {
		options.ChownDirs = item.idPair
		options.ChownFiles = item.idPair
	}
	return options
}

// getOptions returns GET options for buildah's copier package.
func (item *CopyItem) getOptions() buildahCopiah.GetOptions {
	options := buildahCopiah.GetOptions{}
	if item.idMappings != nil {
		options.UIDMap = item.idMappings.UIDMap
		options.GIDMap = item.idMappings.GIDMap
	}
	if item.idPair != nil {
		options.ChownDirs = item.idPair
		options.ChownFiles = item.idPair
	}
	return options

}

// mount and pause the container.  Also set the item's cleanUpFuncs.  Those
// *must* be invoked by callers, even in case of errors.
func (item *CopyItem) mountAndPauseContainer(container *libpod.Container, pause bool) (string, error) {
	// Make sure to pause and unpause the container.  We cannot pause on
	// cgroupsv1 as rootless user, in which case we turn off pausing.
	if pause && rootless.IsRootless() {
		cgroupv2, _ := cgroups.IsCgroup2UnifiedMode()
		if !cgroupv2 {
			logrus.Debugf("Cannot pause container for copying as a rootless user on cgroupsv1: default to not pause")
			pause = false
		}
	}

	// Mount and unmount the container.
	mountPoint, err := container.Mount()
	if err != nil {
		return "", err
	}

	item.cleanUpFuncs = append(item.cleanUpFuncs, func() {
		if err := container.Unmount(false); err != nil {
			logrus.Errorf("Error unmounting container after copy operation: %v", err)
		}
	})

	// Pause and unpause the container.
	if pause {
		if err := container.Pause(); err != nil {
			// Ignore errors when the container isn't running.  No
			// need to pause.
			if errors.Cause(err) != define.ErrCtrStateInvalid {
				return "", err
			}
		} else {
			item.cleanUpFuncs = append(item.cleanUpFuncs, func() {
				if err := container.Unpause(); err != nil {
					logrus.Errorf("Error unpausing container after copy operation: %v", err)
				}
			})
		}
	}

	return mountPoint, nil
}

// buildahGlobs returns the root, dir and glob used in buildah's copier
// package.
//
// Note that dir is always empty.
func (item *CopyItem) buildahGlobs() (root string, glob string, err error) {
	root = item.root

	// If the root and the resolved path are equal, then dir must be empty
	// and the glob must be ".".
	if filepath.Clean(root) == filepath.Clean(item.resolved) {
		glob = "."
		return
	}

	glob, err = filepath.Rel(root, item.resolved)
	return
}

// preserveBasePath makes sure that the original base path (e.g., "/" or "./")
// is preserved.  The filepath API among tends to clean up a bit too much but
// we *must* preserve this data by all means.
func preserveBasePath(original, resolved string) string {
	// Handle "/"
	if strings.HasSuffix(original, "/") {
		if !strings.HasSuffix(resolved, "/") {
			resolved += "/"
		}
		return resolved
	}

	// Handle "/."
	if strings.HasSuffix(original, "/.") {
		if strings.HasSuffix(resolved, "/") { // could be root!
			resolved += "."
		} else if !strings.HasSuffix(resolved, "/.") {
			resolved += "/."
		}
		return resolved
	}

	return resolved
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
		return nil, errors.Errorf("internal libpod error: secureStat: expected 1 item but got %d", len(globStats))
	}

	stat, exists := globStats[0].Results[glob] // only one glob passed, so that's okay
	if !exists {
		return stat, os.ErrNotExist
	}

	var statErr error
	if stat.Error != "" {
		statErr = errors.New(stat.Error)
	}
	return stat, statErr
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
	// NOTE: the secure join makes sure that we follow symlinks.  This way,
	// we catch scenarios where the container path symlinks to a volume or
	// bind mount.
	resolvedPathOnTheContainerMountPoint, err := securejoin.SecureJoin(mountPoint, pathRelativeToContainerMountPoint)
	if err != nil {
		return "", "", err
	}
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

// getIDMappingsAndPair returns the ID mappings for the container and the host
// ID pair.
func getIDMappingsAndPair(container *libpod.Container, containerMount string) (*storage.IDMappingOptions, *idtools.IDPair, error) {
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
func getContainerUser(container *libpod.Container, mountPoint string) (specs.User, error) {
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
