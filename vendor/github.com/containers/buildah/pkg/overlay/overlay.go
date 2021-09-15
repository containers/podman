package overlay

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/system"
	"github.com/containers/storage/pkg/unshare"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// TempDir generates an overlay Temp directory in the container content
func TempDir(containerDir string, rootUID, rootGID int) (string, error) {
	contentDir := filepath.Join(containerDir, "overlay")
	if err := idtools.MkdirAllAs(contentDir, 0700, rootUID, rootGID); err != nil {
		return "", errors.Wrapf(err, "failed to create the overlay %s directory", contentDir)
	}

	contentDir, err := ioutil.TempDir(contentDir, "")
	if err != nil {
		return "", errors.Wrapf(err, "failed to create the overlay tmpdir in %s directory", contentDir)
	}

	return generateOverlayStructure(contentDir, rootUID, rootGID)
}

// GenerateStructure generates an overlay directory structure for container content
func GenerateStructure(containerDir, containerID, name string, rootUID, rootGID int) (string, error) {
	contentDir := filepath.Join(containerDir, "overlay-containers", containerID, name)
	if err := idtools.MkdirAllAs(contentDir, 0700, rootUID, rootGID); err != nil {
		return "", errors.Wrapf(err, "failed to create the overlay %s directory", contentDir)
	}

	return generateOverlayStructure(contentDir, rootUID, rootGID)
}

// generateOverlayStructure generates upper, work and merge directory structure for overlay directory
func generateOverlayStructure(containerDir string, rootUID, rootGID int) (string, error) {
	upperDir := filepath.Join(containerDir, "upper")
	workDir := filepath.Join(containerDir, "work")
	if err := idtools.MkdirAllAs(upperDir, 0700, rootUID, rootGID); err != nil {
		return "", errors.Wrapf(err, "failed to create the overlay %s directory", upperDir)
	}
	if err := idtools.MkdirAllAs(workDir, 0700, rootUID, rootGID); err != nil {
		return "", errors.Wrapf(err, "failed to create the overlay %s directory", workDir)
	}
	mergeDir := filepath.Join(containerDir, "merge")
	if err := idtools.MkdirAllAs(mergeDir, 0700, rootUID, rootGID); err != nil {
		return "", errors.Wrapf(err, "failed to create the overlay %s directory", mergeDir)
	}

	return containerDir, nil
}

// Mount creates a subdir of the contentDir based on the source directory
// from the source system.  It then mounts up the source directory on to the
// generated mount point and returns the mount point to the caller.
func Mount(contentDir, source, dest string, rootUID, rootGID int, graphOptions []string) (mount specs.Mount, Err error) {
	return mountHelper(contentDir, source, dest, rootUID, rootGID, graphOptions, false)
}

// MountReadOnly creates a subdir of the contentDir based on the source directory
// from the source system.  It then mounts up the source directory on to the
// generated mount point and returns the mount point to the caller.  Note that no
// upper layer will be created rendering it a read-only mount
func MountReadOnly(contentDir, source, dest string, rootUID, rootGID int, graphOptions []string) (mount specs.Mount, Err error) {
	return mountHelper(contentDir, source, dest, rootUID, rootGID, graphOptions, true)
}

// NOTE: rootUID and rootUID are not yet used.
func mountHelper(contentDir, source, dest string, _, _ int, graphOptions []string, readOnly bool) (mount specs.Mount, Err error) {
	mergeDir := filepath.Join(contentDir, "merge")

	// Create overlay mount options for rw/ro.
	var overlayOptions string
	if readOnly {
		// Read-only overlay mounts require two lower layer.
		lowerTwo := filepath.Join(contentDir, "lower")
		if err := os.Mkdir(lowerTwo, 0755); err != nil {
			return mount, err
		}
		overlayOptions = fmt.Sprintf("lowerdir=%s:%s,private", source, lowerTwo)
	} else {
		// Read-write overlay mounts want a lower, upper and a work layer.
		workDir := filepath.Join(contentDir, "work")
		upperDir := filepath.Join(contentDir, "upper")
		st, err := os.Stat(source)
		if err != nil {
			return mount, err
		}
		if err := os.Chmod(upperDir, st.Mode()); err != nil {
			return mount, err
		}
		if stat, ok := st.Sys().(*syscall.Stat_t); ok {
			if err := os.Chown(upperDir, int(stat.Uid), int(stat.Gid)); err != nil {
				return mount, err
			}
		}

		overlayOptions = fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s,private", source, upperDir, workDir)
	}

	if unshare.IsRootless() {
		mountProgram := ""

		mountMap := map[string]bool{
			".mount_program":         true,
			"overlay.mount_program":  true,
			"overlay2.mount_program": true,
		}

		for _, i := range graphOptions {
			s := strings.SplitN(i, "=", 2)
			if len(s) != 2 {
				continue
			}
			key := s[0]
			val := s[1]
			if mountMap[key] {
				mountProgram = val
				break
			}
		}
		if mountProgram != "" {
			cmd := exec.Command(mountProgram, "-o", overlayOptions, mergeDir)

			if err := cmd.Run(); err != nil {
				return mount, errors.Wrapf(err, "exec %s", mountProgram)
			}

			mount.Source = mergeDir
			mount.Destination = dest
			mount.Type = "bind"
			mount.Options = []string{"bind", "slave"}
			return mount, nil
		}
		/* If a mount_program is not specified, fallback to try mount native overlay.  */
		overlayOptions = fmt.Sprintf("%s,userxattr", overlayOptions)
	}

	mount.Source = mergeDir
	mount.Destination = dest
	mount.Type = "overlay"
	mount.Options = strings.Split(overlayOptions, ",")

	return mount, nil
}

// RemoveTemp removes temporary mountpoint and all content from its parent
// directory
func RemoveTemp(contentDir string) error {
	if err := Unmount(contentDir); err != nil {
		return err
	}

	return os.RemoveAll(contentDir)
}

// Unmount the overlay mountpoint
func Unmount(contentDir string) error {
	mergeDir := filepath.Join(contentDir, "merge")

	if unshare.IsRootless() {
		// Attempt to unmount the FUSE mount using either fusermount or fusermount3.
		// If they fail, fallback to unix.Unmount
		for _, v := range []string{"fusermount3", "fusermount"} {
			err := exec.Command(v, "-u", mergeDir).Run()
			if err != nil && errors.Cause(err) != exec.ErrNotFound {
				logrus.Debugf("Error unmounting %s with %s - %v", mergeDir, v, err)
			}
			if err == nil {
				return nil
			}
		}
		// If fusermount|fusermount3 failed to unmount the FUSE file system, attempt unmount
	}

	// Ignore EINVAL as the specified merge dir is not a mount point
	if err := unix.Unmount(mergeDir, 0); err != nil && !os.IsNotExist(err) && err != unix.EINVAL {
		return errors.Wrapf(err, "unmount overlay %s", mergeDir)
	}
	return nil
}

func recreate(contentDir string) error {
	st, err := system.Stat(contentDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.Wrap(err, "failed to stat overlay upper directory")
	}

	if err := os.RemoveAll(contentDir); err != nil {
		return errors.WithStack(err)
	}

	if err := idtools.MkdirAllAs(contentDir, os.FileMode(st.Mode()), int(st.UID()), int(st.GID())); err != nil {
		return errors.Wrap(err, "failed to create overlay directory")
	}
	return nil
}

// CleanupMount removes all temporary mountpoint content
func CleanupMount(contentDir string) (Err error) {
	if err := recreate(filepath.Join(contentDir, "upper")); err != nil {
		return err
	}
	if err := recreate(filepath.Join(contentDir, "work")); err != nil {
		return err
	}
	return nil
}

// CleanupContent removes all temporary mountpoint and all content from
// directory
func CleanupContent(containerDir string) (Err error) {
	contentDir := filepath.Join(containerDir, "overlay")

	files, err := ioutil.ReadDir(contentDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.Wrap(err, "read directory")
	}
	for _, f := range files {
		dir := filepath.Join(contentDir, f.Name())
		if err := Unmount(dir); err != nil {
			return err
		}
	}

	if err := os.RemoveAll(contentDir); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to cleanup overlay directory")
	}
	return nil
}
