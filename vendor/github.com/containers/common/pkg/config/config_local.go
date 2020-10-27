// +build !remote

package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	units "github.com/docker/go-units"
	"github.com/pkg/errors"
)

// isDirectory tests whether the given path exists and is a directory. It
// follows symlinks.
func isDirectory(path string) error {
	path, err := resolveHomeDir(path)
	if err != nil {
		return err
	}

	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if !info.Mode().IsDir() {
		// Return a PathError to be consistent with os.Stat().
		return &os.PathError{
			Op:   "stat",
			Path: path,
			Err:  syscall.ENOTDIR,
		}
	}

	return nil
}

func (c *EngineConfig) validatePaths() error {
	// Relative paths can cause nasty bugs, because core paths we use could
	// shift between runs or even parts of the program. - The OCI runtime
	// uses a different working directory than we do, for example.
	if c.StaticDir != "" && !filepath.IsAbs(c.StaticDir) {
		return errors.Errorf("static directory must be an absolute path - instead got %q", c.StaticDir)
	}
	if c.TmpDir != "" && !filepath.IsAbs(c.TmpDir) {
		return errors.Errorf("temporary directory must be an absolute path - instead got %q", c.TmpDir)
	}
	if c.VolumePath != "" && !filepath.IsAbs(c.VolumePath) {
		return errors.Errorf("volume path must be an absolute path - instead got %q", c.VolumePath)
	}
	return nil
}

func (c *ContainersConfig) validateDevices() error {
	for _, d := range c.Devices {
		_, _, _, err := Device(d)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *ContainersConfig) validateUlimits() error {
	for _, u := range c.DefaultUlimits {
		ul, err := units.ParseUlimit(u)
		if err != nil {
			return errors.Wrapf(err, "unrecognized ulimit %s", u)
		}
		_, err = ul.GetRlimit()
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *ContainersConfig) validateTZ() error {
	if c.TZ == "local" || c.TZ == "" {
		return nil
	}

	lookupPaths := []string{
		"/usr/share/zoneinfo",
		"/etc/zoneinfo",
	}

	for _, paths := range lookupPaths {
		zonePath := filepath.Join(paths, c.TZ)
		if _, err := os.Stat(zonePath); err == nil {
			// found zone information
			return nil
		}
	}

	return errors.Errorf(
		"find timezone %s in paths: %s",
		c.TZ, strings.Join(lookupPaths, ", "),
	)
}

func (c *ContainersConfig) validateUmask() error {
	validUmask := regexp.MustCompile(`^[0-7]{1,4}$`)
	if !validUmask.MatchString(c.Umask) {
		return errors.Errorf("not a valid umask %s", c.Umask)
	}
	return nil
}

func isRemote() bool {
	return false
}
