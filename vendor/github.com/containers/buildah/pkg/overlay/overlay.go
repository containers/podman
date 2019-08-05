package overlay

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/containers/buildah/pkg/unshare"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/idtools"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// MountTemp creates a subdir of the contentDir based on the source directory
// from the source system.  It then mounts up the source directory on to the
// generated mount point and returns the mount point to the caller.
func MountTemp(store storage.Store, containerID, source, dest string, rootUID, rootGID int) (mount specs.Mount, contentDir string, Err error) {

	containerDir, err := store.ContainerDirectory(containerID)
	if err != nil {
		return mount, "", err
	}
	contentDir = filepath.Join(containerDir, "overlay")
	if err := idtools.MkdirAllAs(contentDir, 0700, rootUID, rootGID); err != nil {
		return mount, "", errors.Wrapf(err, "failed to create the overlay %s directory", contentDir)
	}

	contentDir, err = ioutil.TempDir(contentDir, "")
	if err != nil {
		return mount, "", errors.Wrapf(err, "failed to create TempDir in the overlay %s directory", contentDir)
	}
	defer func() {
		if Err != nil {
			os.RemoveAll(contentDir)
		}
	}()

	upperDir := filepath.Join(contentDir, "upper")
	workDir := filepath.Join(contentDir, "work")
	if err := idtools.MkdirAllAs(upperDir, 0700, rootUID, rootGID); err != nil {
		return mount, "", errors.Wrapf(err, "failed to create the overlay %s directory", upperDir)
	}
	if err := idtools.MkdirAllAs(workDir, 0700, rootUID, rootGID); err != nil {
		return mount, "", errors.Wrapf(err, "failed to create the overlay %s directory", workDir)
	}

	overlayOptions := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s,private", source, upperDir, workDir)

	if unshare.IsRootless() {
		mountProgram := ""

		mountMap := map[string]bool{
			".mount_program":         true,
			"overlay.mount_program":  true,
			"overlay2.mount_program": true,
		}

		for _, i := range store.GraphOptions() {
			s := strings.SplitN(i, "=", 2)
			if len(s) != 2 {
				continue
			}
			k := s[0]
			v := s[1]
			if mountMap[k] {
				mountProgram = v
				break
			}
		}
		if mountProgram != "" {
			mergeDir := filepath.Join(contentDir, "merge")

			if err := idtools.MkdirAllAs(mergeDir, 0700, rootUID, rootGID); err != nil {
				return mount, "", errors.Wrapf(err, "failed to create the overlay %s directory", mergeDir)
			}

			cmd := exec.Command(mountProgram, "-o", overlayOptions, mergeDir)

			if err := cmd.Run(); err != nil {
				return mount, "", errors.Wrapf(err, "exec %s", mountProgram)
			}

			mount.Source = mergeDir
			mount.Destination = dest
			mount.Type = "bind"
			mount.Options = []string{"bind", "slave"}
			return mount, contentDir, nil
		}
		/* If a mount_program is not specified, fallback to try mount native overlay.  */
	}

	mount.Source = "overlay"
	mount.Destination = dest
	mount.Type = "overlay"
	mount.Options = strings.Split(overlayOptions, ",")

	return mount, contentDir, nil
}

// RemoveTemp removes temporary mountpoint and all content from its parent
// directory
func RemoveTemp(contentDir string) error {
	if unshare.IsRootless() {
		mergeDir := filepath.Join(contentDir, "merge")
		if err := unix.Unmount(mergeDir, 0); err != nil {
			if !os.IsNotExist(err) {
				return errors.Wrapf(err, "unmount overlay %s", mergeDir)
			}
		}
	}
	return os.RemoveAll(contentDir)
}

// CleanupContent removes all temporary mountpoint and all content from
// directory
func CleanupContent(containerDir string) (Err error) {
	contentDir := filepath.Join(containerDir, "overlay")

	if unshare.IsRootless() {
		mergeDir := filepath.Join(contentDir, "merge")
		if err := unix.Unmount(mergeDir, 0); err != nil {
			if !os.IsNotExist(err) {
				return errors.Wrapf(err, "unmount overlay %s", mergeDir)
			}
		}
	}
	if err := os.RemoveAll(contentDir); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed to cleanup overlay %s directory", contentDir)
	}
	return nil
}
