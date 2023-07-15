//go:build linux && composefs && cgo
// +build linux,composefs,cgo

package overlay

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"

	"github.com/containers/storage/pkg/loopback"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

var (
	composeFsHelperOnce sync.Once
	composeFsHelperPath string
	composeFsHelperErr  error
)

func getComposeFsHelper() (string, error) {
	composeFsHelperOnce.Do(func() {
		composeFsHelperPath, composeFsHelperErr = exec.LookPath("composefs-from-json")
	})
	return composeFsHelperPath, composeFsHelperErr
}

func composeFsSupported() bool {
	_, err := getComposeFsHelper()
	return err == nil
}

func enableVerity(description string, fd int) error {
	enableArg := unix.FsverityEnableArg{
		Version:        1,
		Hash_algorithm: unix.FS_VERITY_HASH_ALG_SHA256,
		Block_size:     4096,
	}

	_, _, e1 := syscall.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(unix.FS_IOC_ENABLE_VERITY), uintptr(unsafe.Pointer(&enableArg)))
	if e1 != 0 && !errors.Is(e1, unix.EEXIST) {
		return fmt.Errorf("failed to enable verity for %q: %w", description, e1)
	}
	return nil
}

func enableVerityRecursive(path string) error {
	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		if err := enableVerity(path, int(f.Fd())); err != nil {
			return err
		}
		return nil
	}
	return filepath.WalkDir(path, walkFn)
}

func getComposefsBlob(dataDir string) string {
	return filepath.Join(dataDir, "composefs.blob")
}

func generateComposeFsBlob(toc []byte, composefsDir string) error {
	if err := os.MkdirAll(composefsDir, 0o700); err != nil {
		return err
	}

	destFile := getComposefsBlob(composefsDir)
	writerJson, err := getComposeFsHelper()
	if err != nil {
		return fmt.Errorf("failed to find composefs-from-json: %w", err)
	}

	fd, err := unix.Openat(unix.AT_FDCWD, destFile, unix.O_WRONLY|unix.O_CREAT|unix.O_TRUNC|unix.O_EXCL|unix.O_CLOEXEC, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open output file: %w", err)
	}
	outFd := os.NewFile(uintptr(fd), "outFd")

	fd, err = unix.Open(fmt.Sprintf("/proc/self/fd/%d", outFd.Fd()), unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		outFd.Close()
		return fmt.Errorf("failed to dup output file: %w", err)
	}
	newFd := os.NewFile(uintptr(fd), "newFd")
	defer newFd.Close()

	err = func() error {
		// a scope to close outFd before setting fsverity on the read-only fd.
		defer outFd.Close()

		cmd := exec.Command(writerJson, "--format=erofs", "--out=/proc/self/fd/3", "/proc/self/fd/0")
		cmd.ExtraFiles = []*os.File{outFd}
		cmd.Stderr = os.Stderr
		cmd.Stdin = bytes.NewReader(toc)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to convert json to erofs: %w", err)
		}
		return nil
	}()
	if err != nil {
		return err
	}

	if err := enableVerity("manifest file", int(newFd.Fd())); err != nil && !errors.Is(err, unix.ENOTSUP) && !errors.Is(err, unix.ENOTTY) {
		logrus.Warningf("%s", err)
	}

	return nil
}

/*
typedef enum {
	LCFS_EROFS_FLAGS_HAS_ACL = (1 << 0),
} lcfs_erofs_flag_t;

struct lcfs_erofs_header_s {
	uint32_t magic;
	uint32_t version;
	uint32_t flags;
	uint32_t unused[5];
} __attribute__((__packed__));
*/

// hasACL returns true if the erofs blob has ACLs enabled
func hasACL(path string) (bool, error) {
	const LCFS_EROFS_FLAGS_HAS_ACL = (1 << 0)

	fd, err := unix.Openat(unix.AT_FDCWD, path, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return false, err
	}
	defer unix.Close(fd)
	// do not worry about checking the magic number, if the file is invalid
	// we will fail to mount it anyway
	flags := make([]byte, 4)
	nread, err := unix.Pread(fd, flags, 8)
	if err != nil {
		return false, err
	}
	if nread != 4 {
		return false, fmt.Errorf("failed to read flags from %q", path)
	}
	return binary.LittleEndian.Uint32(flags)&LCFS_EROFS_FLAGS_HAS_ACL != 0, nil
}

func mountComposefsBlob(dataDir, mountPoint string) error {
	blobFile := getComposefsBlob(dataDir)
	loop, err := loopback.AttachLoopDevice(blobFile)
	if err != nil {
		return err
	}
	defer loop.Close()

	hasACL, err := hasACL(blobFile)
	if err != nil {
		return err
	}
	mountOpts := "ro"
	if !hasACL {
		mountOpts += ",noacl"
	}

	return unix.Mount(loop.Name(), mountPoint, "erofs", unix.MS_RDONLY, mountOpts)
}
