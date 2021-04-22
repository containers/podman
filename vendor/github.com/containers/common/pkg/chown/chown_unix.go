// +build !windows

package chown

import (
	"os"
	"path/filepath"
	"syscall"

	"github.com/pkg/errors"
)

// ChangeHostPathOwnership changes the uid and gid ownership of a directory or file within the host.
// This is used by the volume U flag to change source volumes ownership
func ChangeHostPathOwnership(path string, recursive bool, uid, gid int) error {
	// Validate if host path can be chowned
	isDangerous, err := DangerousHostPath(path)
	if err != nil {
		return errors.Wrap(err, "failed to validate if host path is dangerous")
	}

	if isDangerous {
		return errors.Errorf("chowning host path %q is not allowed. You can manually `chown -R %d:%d %s`", path, uid, gid, path)
	}

	// Chown host path
	if recursive {
		err := filepath.Walk(path, func(filePath string, f os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Get current ownership
			currentUID := int(f.Sys().(*syscall.Stat_t).Uid)
			currentGID := int(f.Sys().(*syscall.Stat_t).Gid)

			if uid != currentUID || gid != currentGID {
				return os.Lchown(filePath, uid, gid)
			}

			return nil
		})

		if err != nil {
			return errors.Wrap(err, "failed to chown recursively host path")
		}
	} else {
		// Get host path info
		f, err := os.Lstat(path)
		if err != nil {
			return errors.Wrap(err, "failed to get host path information")
		}

		// Get current ownership
		currentUID := int(f.Sys().(*syscall.Stat_t).Uid)
		currentGID := int(f.Sys().(*syscall.Stat_t).Gid)

		if uid != currentUID || gid != currentGID {
			if err := os.Lchown(path, uid, gid); err != nil {
				return errors.Wrap(err, "failed to chown host path")
			}
		}
	}

	return nil
}
