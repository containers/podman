package util

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"github.com/containers/psgo"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// GetContainerPidInformationDescriptors returns a string slice of all supported
// format descriptors of GetContainerPidInformation.
func GetContainerPidInformationDescriptors() ([]string, error) {
	return psgo.ListDescriptors(), nil
}

// FindDeviceNodes parses /dev/ into a set of major:minor -> path, where
// [major:minor] is the device's major and minor numbers formatted as, for
// example, 2:0 and path is the path to the device node.
// Symlinks to nodes are ignored.
func FindDeviceNodes() (map[string]string, error) {
	nodes := make(map[string]string)
	err := filepath.WalkDir("/dev", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			logrus.Warnf("Error descending into path %s: %v", path, err)
			return filepath.SkipDir
		}

		// If we aren't a device node, do nothing.
		if d.Type()&(os.ModeDevice|os.ModeCharDevice) == 0 {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		// We are a device node. Get major/minor.
		sysstat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return errors.Errorf("Could not convert stat output for use")
		}
		// We must typeconvert sysstat.Rdev from uint64->int to avoid constant overflow
		rdev := int(sysstat.Rdev)
		major := ((rdev >> 8) & 0xfff) | ((rdev >> 32) & ^0xfff)
		minor := (rdev & 0xff) | ((rdev >> 12) & ^0xff)

		nodes[fmt.Sprintf("%d:%d", major, minor)] = path

		return nil
	})
	if err != nil {
		return nil, err
	}

	return nodes, nil
}
