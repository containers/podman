// +build systemd

package config

import (
	"github.com/containers/common/pkg/cgroupv2"
	"github.com/containers/storage/pkg/unshare"
)

func defaultCgroupManager() string {
	enabled, err := cgroupv2.Enabled()
	if err == nil && !enabled && unshare.IsRootless() {
		return CgroupfsCgroupsManager
	}

	return SystemdCgroupsManager
}
func defaultEventsLogger() string {
	return "journald"
}
