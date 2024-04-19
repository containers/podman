//go:build !remote

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/containers/storage/pkg/fileutils"
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

func (c *ContainersConfig) validateInterfaceName() error {
	if c.InterfaceName == "device" || c.InterfaceName == "" {
		return nil
	}

	return fmt.Errorf("invalid interface_name option %s", c.InterfaceName)
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

	// Allow using TZDIR to override the lookupPaths. Ref:
	// https://sourceware.org/git/?p=glibc.git;a=blob;f=time/tzfile.c;h=8a923d0cccc927a106dc3e3c641be310893bab4e;hb=HEAD#l149
	tzdir := os.Getenv("TZDIR")
	if tzdir != "" {
		lookupPaths = []string{tzdir}
	}

	for _, paths := range lookupPaths {
		zonePath := filepath.Join(paths, c.TZ)
		if err := fileutils.Exists(zonePath); err == nil {
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
