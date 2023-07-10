//go:build !windows
// +build !windows

package archive

import (
	"archive/tar"
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/system"
	"golang.org/x/sys/unix"
)

// fixVolumePathPrefix does platform specific processing to ensure that if
// the path being passed in is not in a volume path format, convert it to one.
func fixVolumePathPrefix(srcPath string) string {
	return srcPath
}

// getWalkRoot calculates the root path when performing a TarWithOptions.
// We use a separate function as this is platform specific. On Linux, we
// can't use filepath.Join(srcPath,include) because this will clean away
// a trailing "." or "/" which may be important.
func getWalkRoot(srcPath string, include string) string {
	return srcPath + string(filepath.Separator) + include
}

// CanonicalTarNameForPath returns platform-specific filepath
// to canonical posix-style path for tar archival. p is relative
// path.
func CanonicalTarNameForPath(p string) (string, error) {
	return p, nil // already unix-style
}

// chmodTarEntry is used to adjust the file permissions used in tar header based
// on the platform the archival is done.

func chmodTarEntry(perm os.FileMode) os.FileMode {
	return perm // noop for unix as golang APIs provide perm bits correctly
}

func setHeaderForSpecialDevice(hdr *tar.Header, name string, stat interface{}) (err error) {
	s, ok := stat.(*syscall.Stat_t)

	if ok {
		// Currently go does not fill in the major/minors
		if s.Mode&unix.S_IFBLK != 0 ||
			s.Mode&unix.S_IFCHR != 0 {
			hdr.Devmajor = int64(major(uint64(s.Rdev))) //nolint: unconvert
			hdr.Devminor = int64(minor(uint64(s.Rdev))) //nolint: unconvert
		}
	}

	return
}

func getInodeFromStat(stat interface{}) (inode uint64, err error) {
	s, ok := stat.(*syscall.Stat_t)

	if ok {
		inode = s.Ino
	}

	return
}

func getFileUIDGID(stat interface{}) (idtools.IDPair, error) {
	s, ok := stat.(*syscall.Stat_t)

	if !ok {
		return idtools.IDPair{}, errors.New("cannot convert stat value to syscall.Stat_t")
	}
	return idtools.IDPair{UID: int(s.Uid), GID: int(s.Gid)}, nil
}

func major(device uint64) uint64 {
	return (device >> 8) & 0xfff
}

func minor(device uint64) uint64 {
	return (device & 0xff) | ((device >> 12) & 0xfff00)
}

// handleTarTypeBlockCharFifo is an OS-specific helper function used by
// createTarFile to handle the following types of header: Block; Char; Fifo
func handleTarTypeBlockCharFifo(hdr *tar.Header, path string) error {
	mode := uint32(hdr.Mode & 0o7777)
	switch hdr.Typeflag {
	case tar.TypeBlock:
		mode |= unix.S_IFBLK
	case tar.TypeChar:
		mode |= unix.S_IFCHR
	case tar.TypeFifo:
		mode |= unix.S_IFIFO
	}

	return system.Mknod(path, mode, system.Mkdev(hdr.Devmajor, hdr.Devminor))
}

// Hardlink without symlinks
func handleLLink(targetPath, path string) error {
	// Note: on Linux, the link syscall will not follow symlinks.
	// This behavior is implementation-dependent since
	// POSIX.1-2008 so to make it clear that we need non-symlink
	// following here we use the linkat syscall which has a flags
	// field to select symlink following or not.
	return unix.Linkat(unix.AT_FDCWD, targetPath, unix.AT_FDCWD, path, 0)
}
