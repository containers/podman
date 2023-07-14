package overlay

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"errors"

	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/system"
	"github.com/containers/storage/pkg/unshare"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// Options type holds various configuration options for overlay
// MountWithOptions accepts following type so it is easier to specify
// more verbose configuration for overlay mount.
type Options struct {
	// The Upper directory is normally writable layer in an overlay mount.
	// Note!! : Following API does not handles escaping or validates correctness of the values
	// passed to UpperDirOptionFragment instead API will try to pass values as is it
	// to the `mount` command. It is user's responsibility to make sure they pre-validate
	// these values. Invalid inputs may lead to undefined behaviour.
	// This is provided as-is, use it if it works for you, we can/will change/break that in the future.
	// See discussion here for more context: https://github.com/containers/buildah/pull/3715#discussion_r786036959
	// TODO: Should we address above comment and handle escaping of metacharacters like
	// `comma`, `backslash` ,`colon` and any other special characters
	UpperDirOptionFragment string
	// The Workdir is used to prepare files as they are switched between the layers.
	// Note!! : Following API does not handles escaping or validates correctness of the values
	// passed to WorkDirOptionFragment instead API will try to pass values as is it
	// to the `mount` command. It is user's responsibility to make sure they pre-validate
	// these values. Invalid inputs may lead to undefined behaviour.
	// This is provided as-is, use it if it works for you, we can/will change/break that in the future.
	// See discussion here for more context: https://github.com/containers/buildah/pull/3715#discussion_r786036959
	// TODO: Should we address above comment and handle escaping of metacharacters like
	// `comma`, `backslash` ,`colon` and any other special characters
	WorkDirOptionFragment string
	// Graph options relayed from podman, will be responsible for choosing mount program
	GraphOpts []string
	// Mark if following overlay is read only
	ReadOnly bool
	// RootUID is not used yet but keeping it here for legacy reasons.
	RootUID int
	// RootGID is not used yet but keeping it here for legacy reasons.
	RootGID int
}

// TempDir generates an overlay Temp directory in the container content
func TempDir(containerDir string, rootUID, rootGID int) (string, error) {
	contentDir := filepath.Join(containerDir, "overlay")
	if err := idtools.MkdirAllAs(contentDir, 0700, rootUID, rootGID); err != nil {
		return "", fmt.Errorf("failed to create the overlay %s directory: %w", contentDir, err)
	}

	contentDir, err := os.MkdirTemp(contentDir, "")
	if err != nil {
		return "", fmt.Errorf("failed to create the overlay tmpdir in %s directory: %w", contentDir, err)
	}

	return generateOverlayStructure(contentDir, rootUID, rootGID)
}

// GenerateStructure generates an overlay directory structure for container content
func GenerateStructure(containerDir, containerID, name string, rootUID, rootGID int) (string, error) {
	contentDir := filepath.Join(containerDir, "overlay-containers", containerID, name)
	if err := idtools.MkdirAllAs(contentDir, 0700, rootUID, rootGID); err != nil {
		return "", fmt.Errorf("failed to create the overlay %s directory: %w", contentDir, err)
	}

	return generateOverlayStructure(contentDir, rootUID, rootGID)
}

// generateOverlayStructure generates upper, work and merge directory structure for overlay directory
func generateOverlayStructure(containerDir string, rootUID, rootGID int) (string, error) {
	upperDir := filepath.Join(containerDir, "upper")
	workDir := filepath.Join(containerDir, "work")
	if err := idtools.MkdirAllAs(upperDir, 0700, rootUID, rootGID); err != nil {
		return "", fmt.Errorf("failed to create the overlay %s directory: %w", upperDir, err)
	}
	if err := idtools.MkdirAllAs(workDir, 0700, rootUID, rootGID); err != nil {
		return "", fmt.Errorf("failed to create the overlay %s directory: %w", workDir, err)
	}
	mergeDir := filepath.Join(containerDir, "merge")
	if err := idtools.MkdirAllAs(mergeDir, 0700, rootUID, rootGID); err != nil {
		return "", fmt.Errorf("failed to create the overlay %s directory: %w", mergeDir, err)
	}

	return containerDir, nil
}

// Mount creates a subdir of the contentDir based on the source directory
// from the source system.  It then mounts up the source directory on to the
// generated mount point and returns the mount point to the caller.
func Mount(contentDir, source, dest string, rootUID, rootGID int, graphOptions []string) (mount specs.Mount, Err error) {
	overlayOpts := Options{GraphOpts: graphOptions, ReadOnly: false, RootUID: rootUID, RootGID: rootGID}
	return MountWithOptions(contentDir, source, dest, &overlayOpts)
}

// MountReadOnly creates a subdir of the contentDir based on the source directory
// from the source system.  It then mounts up the source directory on to the
// generated mount point and returns the mount point to the caller.  Note that no
// upper layer will be created rendering it a read-only mount
func MountReadOnly(contentDir, source, dest string, rootUID, rootGID int, graphOptions []string) (mount specs.Mount, Err error) {
	overlayOpts := Options{GraphOpts: graphOptions, ReadOnly: true, RootUID: rootUID, RootGID: rootGID}
	return MountWithOptions(contentDir, source, dest, &overlayOpts)
}

// findMountProgram finds if any mount program is specified in the graph options.
func findMountProgram(graphOptions []string) string {
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
			return val
		}
	}

	return ""
}

// mountWithMountProgram mount an overlay at mergeDir using the specified mount program
// and overlay options.
func mountWithMountProgram(mountProgram, overlayOptions, mergeDir string) error {
	cmd := exec.Command(mountProgram, "-o", overlayOptions, mergeDir)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exec %s: %w", mountProgram, err)
	}
	return nil
}

// Convert ":" to "\:", the path which will be overlay mounted need to be escaped
func escapeColon(source string) string {
	return strings.ReplaceAll(source, ":", "\\:")
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
			if err != nil && !errors.Is(err, exec.ErrNotFound) {
				logrus.Debugf("Error unmounting %s with %s - %v", mergeDir, v, err)
			}
			if err == nil {
				return nil
			}
		}
		// If fusermount|fusermount3 failed to unmount the FUSE file system, attempt unmount
	}

	// Ignore EINVAL as the specified merge dir is not a mount point
	if err := unix.Unmount(mergeDir, 0); err != nil && !errors.Is(err, os.ErrNotExist) && err != unix.EINVAL {
		return fmt.Errorf("unmount overlay %s: %w", mergeDir, err)
	}
	return nil
}

func recreate(contentDir string) error {
	st, err := system.Stat(contentDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("failed to stat overlay upper directory: %w", err)
	}

	if err := os.RemoveAll(contentDir); err != nil {
		return err
	}

	if err := idtools.MkdirAllAs(contentDir, os.FileMode(st.Mode()), int(st.UID()), int(st.GID())); err != nil {
		return fmt.Errorf("failed to create overlay directory: %w", err)
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

	files, err := os.ReadDir(contentDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read directory: %w", err)
	}
	for _, f := range files {
		dir := filepath.Join(contentDir, f.Name())
		if err := Unmount(dir); err != nil {
			return err
		}
	}

	if err := os.RemoveAll(contentDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to cleanup overlay directory: %w", err)
	}
	return nil
}
