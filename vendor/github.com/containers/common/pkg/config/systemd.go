// +build systemd,cgo

package config

import (
	"io/ioutil"
	"strings"
	"sync"

	"github.com/containers/common/pkg/cgroupv2"
	"github.com/containers/storage/pkg/unshare"
	"github.com/coreos/go-systemd/v22/sdjournal"
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
		return
	})
	return usesSystemd
}

func useJournald() bool {
	journaldOnce.Do(func() {
		if !useSystemd() {
			return
		}
		journal, err := sdjournal.NewJournal()
		if err != nil {
			return
		}
		journal.Close()
		usesJournald = true
		return
	})
	return usesJournald
}
