package archive

import (
	"archive/tar"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/system"
	"golang.org/x/sys/unix"
)

func getWhiteoutConverter(format WhiteoutFormat, data interface{}) tarWhiteoutConverter {
	if format == OverlayWhiteoutFormat {
		if rolayers, ok := data.([]string); ok && len(rolayers) > 0 {
			return overlayWhiteoutConverter{rolayers: rolayers}
		}
		return overlayWhiteoutConverter{rolayers: nil}
	}
	return nil
}

type overlayWhiteoutConverter struct {
	rolayers []string
}

func (o overlayWhiteoutConverter) ConvertWrite(hdr *tar.Header, path string, fi os.FileInfo) (wo *tar.Header, err error) {
	// convert whiteouts to AUFS format
	if fi.Mode()&os.ModeCharDevice != 0 && hdr.Devmajor == 0 && hdr.Devminor == 0 {
		// we just rename the file and make it normal
		dir, filename := filepath.Split(hdr.Name)
		hdr.Name = filepath.Join(dir, WhiteoutPrefix+filename)
		hdr.Mode = 0600
		hdr.Typeflag = tar.TypeReg
		hdr.Size = 0
	}

	if fi.Mode()&os.ModeDir != 0 {
		// convert opaque dirs to AUFS format by writing an empty file with the whiteout prefix
		opaque, err := system.Lgetxattr(path, "trusted.overlay.opaque")
		if err != nil {
			return nil, err
		}
		if len(opaque) == 1 && opaque[0] == 'y' {
			if hdr.Xattrs != nil {
				delete(hdr.Xattrs, "trusted.overlay.opaque")
			}
			// If there are no lower layers, then it can't have been deleted in this layer.
			if len(o.rolayers) == 0 {
				return nil, nil
			}
			// At this point, we have a directory that's opaque.  If it appears in one of the lower
			// layers, then it was newly-created here, so it wasn't also deleted here.
			for _, rolayer := range o.rolayers {
				stat, statErr := os.Stat(filepath.Join(rolayer, hdr.Name))
				if statErr != nil && !os.IsNotExist(statErr) && !isENOTDIR(statErr) {
					// Not sure what happened here.
					return nil, statErr
				}
				if statErr == nil {
					if stat.Mode()&os.ModeCharDevice != 0 {
						if isWhiteOut(stat) {
							return nil, nil
						}
					}
					// It's not whiteout, so it was there in the older layer, so we need to
					// add a whiteout for this item in this layer.
					// create a header for the whiteout file
					// it should inherit some properties from the parent, but be a regular file
					wo = &tar.Header{
						Typeflag:   tar.TypeReg,
						Mode:       hdr.Mode & int64(os.ModePerm),
						Name:       filepath.Join(hdr.Name, WhiteoutOpaqueDir),
						Size:       0,
						Uid:        hdr.Uid,
						Uname:      hdr.Uname,
						Gid:        hdr.Gid,
						Gname:      hdr.Gname,
						AccessTime: hdr.AccessTime,
						ChangeTime: hdr.ChangeTime,
					}
					break
				}
				for dir := filepath.Dir(hdr.Name); dir != "" && dir != "." && dir != string(os.PathSeparator); dir = filepath.Dir(dir) {
					// Check for whiteout for a parent directory in a parent layer.
					stat, statErr := os.Stat(filepath.Join(rolayer, dir))
					if statErr != nil && !os.IsNotExist(statErr) && !isENOTDIR(statErr) {
						// Not sure what happened here.
						return nil, statErr
					}
					if statErr == nil {
						if stat.Mode()&os.ModeCharDevice != 0 {
							// If it's whiteout for a parent directory, then the
							// original directory wasn't inherited into this layer,
							// so we don't need to emit whiteout for it.
							if isWhiteOut(stat) {
								return nil, nil
							}
						}
					}
				}
			}
		}
	}

	return
}

func (overlayWhiteoutConverter) ConvertRead(hdr *tar.Header, path string) (bool, error) {
	base := filepath.Base(path)
	dir := filepath.Dir(path)

	// if a directory is marked as opaque by the AUFS special file, we need to translate that to overlay
	if base == WhiteoutOpaqueDir {
		err := unix.Setxattr(dir, "trusted.overlay.opaque", []byte{'y'}, 0)
		// don't write the file itself
		return false, err
	}

	// if a file was deleted and we are using overlay, we need to create a character device
	if strings.HasPrefix(base, WhiteoutPrefix) {
		originalBase := base[len(WhiteoutPrefix):]
		originalPath := filepath.Join(dir, originalBase)

		if err := unix.Mknod(originalPath, unix.S_IFCHR, 0); err != nil {
			return false, err
		}
		if err := idtools.SafeChown(originalPath, hdr.Uid, hdr.Gid); err != nil {
			return false, err
		}

		// don't write the file itself
		return false, nil
	}

	return true, nil
}

func isWhiteOut(stat os.FileInfo) bool {
	s := stat.Sys().(*syscall.Stat_t)
	return major(uint64(s.Rdev)) == 0 && minor(uint64(s.Rdev)) == 0
}
