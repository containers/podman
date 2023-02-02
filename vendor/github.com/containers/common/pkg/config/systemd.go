//go:build systemd && cgo
// +build systemd,cgo

package config

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"sync"

	"github.com/containers/common/pkg/cgroupv2"
	"github.com/containers/storage/pkg/unshare"
)

var (
	systemdOnce  sync.Once
	usesSystemd  bool
	journaldOnce sync.Once
	usesJournald bool
)

const (
	// DefaultLogDriver is the default type of log files
	DefaultLogDriver = "journald"
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
	if useJournald() {
		return "journald"
	}
	return "file"
}

func defaultLogDriver() string {
	if useJournald() {
		return "journald"
	}
	return "k8s-file"
}

func useSystemd() bool {
	systemdOnce.Do(func() {
		dat, err := ioutil.ReadFile("/proc/1/comm")
		if err == nil {
			val := strings.TrimSuffix(string(dat), "\n")
			usesSystemd = (val == "systemd")
		}
	})
	return usesSystemd
}

func useJournald() bool {
	journaldOnce.Do(func() {
		if !useSystemd() {
			return
		}
		for _, root := range []string{"/run/log/journal", "/var/log/journal"} {
			dirs, err := ioutil.ReadDir(root)
			if err != nil {
				continue
			}
			for _, d := range dirs {
				if d.IsDir() {
					if _, err := ioutil.ReadDir(filepath.Join(root, d.Name())); err == nil {
						usesJournald = true
						return
					}
				}
			}
		}
	})
	return usesJournald
}
