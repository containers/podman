//go:build linux && !remote

package system

import (
	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/common/pkg/servicereaper"
	"github.com/containers/podman/v5/pkg/rootless"
	"github.com/sirupsen/logrus"
)

// Currently, we only need servicereaper on Linux to support slirp4netns.
func maybeStartServiceReaper() {
	servicereaper.Start()
}

func maybeMoveToSubCgroup() {
	cgroupv2, _ := cgroups.IsCgroup2UnifiedMode()
	if rootless.IsRootless() && !cgroupv2 {
		logrus.Warnf("Running 'system service' in rootless mode without cgroup v2, containers won't survive a 'system service' restart")
	}

	if err := cgroups.MaybeMoveToSubCgroup(); err != nil {
		// it is a best effort operation, so just print the
		// error for debugging purposes.
		logrus.Debugf("Could not move to subcgroup: %v", err)
	}
}
