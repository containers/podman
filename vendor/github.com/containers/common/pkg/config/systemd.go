// +build systemd

package config

import (
	"io/ioutil"
	"strings"
	"sync"

	"github.com/containers/common/pkg/cgroupv2"
	"github.com/containers/storage/pkg/unshare"
)

var (
	systemdOnce sync.Once
	usesSystemd bool
)

func defaultCgroupManager() string {
	if !useSystemd() {
		return CgroupfsCgroupsManager
	}
	enabled, err := cgroupv2.Enabled()
	if err == nil && !enabled && unshare.IsRootless() {
		return CgroupfsCgroupsManager
	}

	return SystemdCgroupsManager
}

func defaultEventsLogger() string {
	if useSystemd() {
		return "journald"
	}
	return "file"
}

func defaultLogDriver() string {
	// If we decide to change the default for logdriver, it should be done here.
	if useSystemd() {
		return DefaultLogDriver
	}

	return DefaultLogDriver

}

func useSystemd() bool {
	systemdOnce.Do(func() {
		dat, err := ioutil.ReadFile("/proc/1/comm")
		if err == nil {
			val := strings.TrimSuffix(string(dat), "\n")
			usesSystemd = (val == "systemd")
		}
		return
	})
	return usesSystemd
}
