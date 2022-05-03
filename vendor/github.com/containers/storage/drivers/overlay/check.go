//go:build linux
// +build linux

package overlay

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"syscall"

	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/ioutils"
	"github.com/containers/storage/pkg/mount"
	"github.com/containers/storage/pkg/system"
	"github.com/containers/storage/pkg/unshare"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// doesSupportNativeDiff checks whether the filesystem has a bug
// which copies up the opaque flag when copying up an opaque
// directory or the kernel enable CONFIG_OVERLAY_FS_REDIRECT_DIR.
// When these exist naive diff should be used.
func doesSupportNativeDiff(d, mountOpts string) error {
	td, err := ioutil.TempDir(d, "opaque-bug-check")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(td); err != nil {
			logrus.Warnf("Failed to remove check directory %v: %v", td, err)
		}
	}()

	// Make directories l1/d, l1/d1, l2/d, l3, work, merged
	if err := os.MkdirAll(filepath.Join(td, "l1", "d"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(td, "l1", "d1"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(td, "l2", "d"), 0755); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(td, "l3"), 0755); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(td, "work"), 0755); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(td, "merged"), 0755); err != nil {
		return err
	}

	// Mark l2/d as opaque
	if err := system.Lsetxattr(filepath.Join(td, "l2", "d"), archive.GetOverlayXattrName("opaque"), []byte("y"), 0); err != nil {
		return errors.Wrap(err, "failed to set opaque flag on middle layer")
	}

	mountFlags := "lowerdir=%s:%s,upperdir=%s,workdir=%s"
	if unshare.IsRootless() {
		mountFlags = mountFlags + ",userxattr"
	}

	opts := fmt.Sprintf(mountFlags, path.Join(td, "l2"), path.Join(td, "l1"), path.Join(td, "l3"), path.Join(td, "work"))
	flags, data := mount.ParseOptions(mountOpts)
	if data != "" {
		opts = fmt.Sprintf("%s,%s", opts, data)
	}
	if err := unix.Mount("overlay", filepath.Join(td, "merged"), "overlay", uintptr(flags), opts); err != nil {
		return errors.Wrap(err, "failed to mount overlay")
	}
	defer func() {
		if err := unix.Unmount(filepath.Join(td, "merged"), 0); err != nil {
			logrus.Warnf("Failed to unmount check directory %v: %v", filepath.Join(td, "merged"), err)
		}
	}()

	// Touch file in d to force copy up of opaque directory "d" from "l2" to "l3"
	if err := ioutil.WriteFile(filepath.Join(td, "merged", "d", "f"), []byte{}, 0644); err != nil {
		return errors.Wrap(err, "failed to write to merged directory")
	}

	// Check l3/d does not have opaque flag
	xattrOpaque, err := system.Lgetxattr(filepath.Join(td, "l3", "d"), archive.GetOverlayXattrName("opaque"))
	if err != nil {
		return errors.Wrap(err, "failed to read opaque flag on upper layer")
	}
	if string(xattrOpaque) == "y" {
		return errors.New("opaque flag erroneously copied up, consider update to kernel 4.8 or later to fix")
	}

	// rename "d1" to "d2"
	if err := os.Rename(filepath.Join(td, "merged", "d1"), filepath.Join(td, "merged", "d2")); err != nil {
		// if rename failed with syscall.EXDEV, the kernel doesn't have CONFIG_OVERLAY_FS_REDIRECT_DIR enabled
		if err.(*os.LinkError).Err == syscall.EXDEV {
			return nil
		}
		return errors.Wrap(err, "failed to rename dir in merged directory")
	}
	// get the xattr of "d2"
	xattrRedirect, err := system.Lgetxattr(filepath.Join(td, "l3", "d2"), archive.GetOverlayXattrName("redirect"))
	if err != nil {
		return errors.Wrap(err, "failed to read redirect flag on upper layer")
	}

	if string(xattrRedirect) == "d1" {
		return errors.New("kernel has CONFIG_OVERLAY_FS_REDIRECT_DIR enabled")
	}

	return nil
}

// doesMetacopy checks if the filesystem is going to optimize changes to
// metadata by using nodes marked with an "overlay.metacopy" attribute to avoid
// copying up a file from a lower layer unless/until its contents are being
// modified
func doesMetacopy(d, mountOpts string) (bool, error) {
	td, err := ioutil.TempDir(d, "metacopy-check")
	if err != nil {
		return false, err
	}
	defer func() {
		if err := os.RemoveAll(td); err != nil {
			logrus.Warnf("Failed to remove check directory %v: %v", td, err)
		}
	}()

	// Make directories l1, l2, work, merged
	if err := os.MkdirAll(filepath.Join(td, "l1"), 0755); err != nil {
		return false, err
	}
	if err := ioutils.AtomicWriteFile(filepath.Join(td, "l1", "f"), []byte{0xff}, 0700); err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Join(td, "l2"), 0755); err != nil {
		return false, err
	}
	if err := os.Mkdir(filepath.Join(td, "work"), 0755); err != nil {
		return false, err
	}
	if err := os.Mkdir(filepath.Join(td, "merged"), 0755); err != nil {
		return false, err
	}
	// Mount using the mandatory options and configured options
	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", path.Join(td, "l1"), path.Join(td, "l2"), path.Join(td, "work"))
	if unshare.IsRootless() {
		opts = fmt.Sprintf("%s,userxattr", opts)
	}
	flags, data := mount.ParseOptions(mountOpts)
	if data != "" {
		opts = fmt.Sprintf("%s,%s", opts, data)
	}
	if err := unix.Mount("overlay", filepath.Join(td, "merged"), "overlay", uintptr(flags), opts); err != nil {
		if errors.Cause(err) == unix.EINVAL {
			logrus.Info("metacopy option not supported on this kernel", mountOpts)
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to mount overlay for metacopy check with %q options", mountOpts)
	}
	defer func() {
		if err := unix.Unmount(filepath.Join(td, "merged"), 0); err != nil {
			logrus.Warnf("Failed to unmount check directory %v: %v", filepath.Join(td, "merged"), err)
		}
	}()
	// Make a change that only impacts the inode, and check if the pulled-up copy is marked
	// as a metadata-only copy
	if err := os.Chmod(filepath.Join(td, "merged", "f"), 0600); err != nil {
		return false, errors.Wrap(err, "error changing permissions on file for metacopy check")
	}
	metacopy, err := system.Lgetxattr(filepath.Join(td, "l2", "f"), archive.GetOverlayXattrName("metacopy"))
	if err != nil {
		if errors.Is(err, unix.ENOTSUP) {
			logrus.Info("metacopy option not supported")
			return false, nil
		}
		return false, errors.Wrap(err, "metacopy flag was not set on file in upper layer")
	}
	return metacopy != nil, nil
}

// doesVolatile checks if the filesystem supports the "volatile" mount option
func doesVolatile(d string) (bool, error) {
	td, err := ioutil.TempDir(d, "volatile-check")
	if err != nil {
		return false, err
	}
	defer func() {
		if err := os.RemoveAll(td); err != nil {
			logrus.Warnf("Failed to remove check directory %v: %v", td, err)
		}
	}()

	if err := os.MkdirAll(filepath.Join(td, "lower"), 0755); err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Join(td, "upper"), 0755); err != nil {
		return false, err
	}
	if err := os.Mkdir(filepath.Join(td, "work"), 0755); err != nil {
		return false, err
	}
	if err := os.Mkdir(filepath.Join(td, "merged"), 0755); err != nil {
		return false, err
	}
	// Mount using the mandatory options and configured options
	opts := fmt.Sprintf("volatile,lowerdir=%s,upperdir=%s,workdir=%s", path.Join(td, "lower"), path.Join(td, "upper"), path.Join(td, "work"))
	if err := unix.Mount("overlay", filepath.Join(td, "merged"), "overlay", 0, opts); err != nil {
		return false, errors.Wrapf(err, "failed to mount overlay for volatile check")
	}
	defer func() {
		if err := unix.Unmount(filepath.Join(td, "merged"), 0); err != nil {
			logrus.Warnf("Failed to unmount check directory %v: %v", filepath.Join(td, "merged"), err)
		}
	}()
	return true, nil
}

// supportsIdmappedLowerLayers checks if the kernel supports mounting overlay on top of
// a idmapped lower layer.
func supportsIdmappedLowerLayers(home string) (bool, error) {
	layerDir, err := ioutil.TempDir(home, "compat")
	if err != nil {
		return false, err
	}
	defer func() {
		_ = os.RemoveAll(layerDir)
	}()

	mergedDir := filepath.Join(layerDir, "merged")
	lowerDir := filepath.Join(layerDir, "lower")
	lowerMappedDir := filepath.Join(layerDir, "lower-mapped")
	upperDir := filepath.Join(layerDir, "upper")
	workDir := filepath.Join(layerDir, "work")

	_ = idtools.MkdirAs(mergedDir, 0700, 0, 0)
	_ = idtools.MkdirAs(lowerDir, 0700, 0, 0)
	_ = idtools.MkdirAs(lowerMappedDir, 0700, 0, 0)
	_ = idtools.MkdirAs(upperDir, 0700, 0, 0)
	_ = idtools.MkdirAs(workDir, 0700, 0, 0)

	idmap := []idtools.IDMap{
		{
			ContainerID: 0,
			HostID:      0,
			Size:        1,
		},
	}
	pid, cleanupFunc, err := createUsernsProcess(idmap, idmap)
	if err != nil {
		return false, err
	}
	defer cleanupFunc()

	if err := createIDMappedMount(lowerDir, lowerMappedDir, int(pid)); err != nil {
		return false, errors.Wrapf(err, "create mapped mount")
	}
	defer unix.Unmount(lowerMappedDir, unix.MNT_DETACH)

	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerMappedDir, upperDir, workDir)
	flags := uintptr(0)
	if err := unix.Mount("overlay", mergedDir, "overlay", flags, opts); err != nil {
		return false, err
	}
	defer func() {
		_ = unix.Unmount(mergedDir, unix.MNT_DETACH)
	}()
	return true, nil
}
