//go:build !remote
// +build !remote

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	units "github.com/docker/go-units"
	"tags.cncf.io/container-device-interface/pkg/parser"
)

func (c *EngineConfig) validatePaths() error {
	// Relative paths can cause nasty bugs, because core paths we use could
	// shift between runs or even parts of the program. - The OCI runtime
	// uses a different working directory than we do, for example.
	if c.StaticDir != "" && !filepath.IsAbs(c.StaticDir) {
		return fmt.Errorf("static directory must be an absolute path - instead got %q", c.StaticDir)
	}
	if c.TmpDir != "" && !filepath.IsAbs(c.TmpDir) {
		return fmt.Errorf("temporary directory must be an absolute path - instead got %q", c.TmpDir)
	}
	if c.VolumePath != "" && !filepath.IsAbs(c.VolumePath) {
		return fmt.Errorf("volume path must be an absolute path - instead got %q", c.VolumePath)
	}
	return nil
}

func (c *ContainersConfig) validateDevices() error {
	for _, d := range c.Devices.Get() {
		if parser.IsQualifiedName(d) {
			continue
		}
		_, _, _, err := Device(d)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *ContainersConfig) validateUlimits() error {
	for _, u := range c.DefaultUlimits.Get() {
		ul, err := units.ParseUlimit(u)
		if err != nil {
			return fmt.Errorf("unrecognized ulimit %s: %w", u, err)
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

	return fmt.Errorf(
		"find timezone %s in paths: %s",
		c.TZ, strings.Join(lookupPaths, ", "),
	)
}

func (c *ContainersConfig) validateUmask() error {
	validUmask := regexp.MustCompile(`^[0-7]{1,4}$`)
	if !validUmask.MatchString(c.Umask) {
		return fmt.Errorf("not a valid umask %s", c.Umask)
	}
	return nil
}

func isRemote() bool {
	return false
}
