package util

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/containers/podman/v2/pkg/rootless"
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
	err := filepath.Walk("/dev", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logrus.Warnf("Error descending into path %s: %v", path, err)
			return filepath.SkipDir
		}

		// If we aren't a device node, do nothing.
		if info.Mode()&(os.ModeDevice|os.ModeCharDevice) == 0 {
			return nil
		}

		// We are a device node. Get major/minor.
		sysstat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return errors.Errorf("Could not convert stat output for use")
		}
		major := sysstat.Rdev / 256
		minor := sysstat.Rdev % 256

		nodes[fmt.Sprintf("%d:%d", major, minor)] = path

		return nil
	})
	if err != nil {
		return nil, err
	}

	return nodes, nil
}

// CheckRootlessUIDRange checks the uid within the rootless container is in the range from /etc/subuid
func CheckRootlessUIDRange(uid int) error {
	uids, _, err := rootless.GetConfiguredMappings()
	if err != nil {
		return err
	}
	total := 0
	for _, u := range uids {
		total += u.Size
	}
	if uid > total {
		return errors.Errorf("requested user's UID %d is too large for the rootless user namespace", uid)
	}
	return nil
}
