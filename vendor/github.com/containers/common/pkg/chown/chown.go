package chown

import (
	"os"
	"os/user"
	"path/filepath"
	"syscall"

	"github.com/containers/storage/pkg/homedir"
	"github.com/pkg/errors"
)

// DangerousHostPath validates if a host path is dangerous and should not be modified
func DangerousHostPath(path string) (bool, error) {
	excludePaths := map[string]bool{
		"/":           true,
		"/bin":        true,
		"/boot":       true,
		"/dev":        true,
		"/etc":        true,
		"/etc/passwd": true,
		"/etc/pki":    true,
		"/etc/shadow": true,
		"/home":       true,
		"/lib":        true,
		"/lib64":      true,
		"/media":      true,
		"/opt":        true,
		"/proc":       true,
		"/root":       true,
		"/run":        true,
		"/sbin":       true,
		"/srv":        true,
		"/sys":        true,
		"/tmp":        true,
		"/usr":        true,
		"/var":        true,
		"/var/lib":    true,
		"/var/log":    true,
	}

	if home := homedir.Get(); home != "" {
		excludePaths[home] = true
	}

	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		if usr, err := user.Lookup(sudoUser); err == nil {
			excludePaths[usr.HomeDir] = true
		}
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return true, err
	}

	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return true, err
	}

	if excludePaths[realPath] {
		return true, nil
	}

	return false, nil
}

// ChangeHostPathOwnership changes the uid and gid ownership of a directory or file within the host.
// This is used by the volume U flag to change source volumes ownership
func ChangeHostPathOwnership(path string, recursive bool, uid, gid int) error {
	// Validate if host path can be chowned
	isDangerous, err := DangerousHostPath(path)
	if err != nil {
		return errors.Wrapf(err, "failed to validate if host path is dangerous")
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
			return errors.Wrapf(err, "failed to chown recursively host path")
		}
	} else {
		// Get host path info
		f, err := os.Lstat(path)
		if err != nil {
			return errors.Wrapf(err, "failed to get host path information")
		}

		// Get current ownership
		currentUID := int(f.Sys().(*syscall.Stat_t).Uid)
		currentGID := int(f.Sys().(*syscall.Stat_t).Gid)

		if uid != currentUID || gid != currentGID {
			if err := os.Lchown(path, uid, gid); err != nil {
				return errors.Wrapf(err, "failed to chown host path")
			}
		}
	}

	return nil
}
