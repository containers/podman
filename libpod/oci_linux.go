// +build linux

package libpod

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/containerd/cgroups"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/projectatomic/libpod/utils"
	"github.com/sirupsen/logrus"
)

func (r *OCIRuntime) moveConmonToCgroup(ctr *Container, cgroupParent string, cmd *exec.Cmd) error {
	if os.Getuid() == 0 {
		if r.cgroupManager == SystemdCgroupsManager {
			unitName := createUnitName("libpod-conmon", ctr.ID())

			logrus.Infof("Running conmon under slice %s and unitName %s", cgroupParent, unitName)
			if err := utils.RunUnderSystemdScope(cmd.Process.Pid, cgroupParent, unitName); err != nil {
				logrus.Warnf("Failed to add conmon to systemd sandbox cgroup: %v", err)
			}
		} else {
			cgroupPath := filepath.Join(ctr.config.CgroupParent, fmt.Sprintf("libpod-%s", ctr.ID()), "conmon")
			control, err := cgroups.New(cgroups.V1, cgroups.StaticPath(cgroupPath), &spec.LinuxResources{})
			if err != nil {
				logrus.Warnf("Failed to add conmon to cgroupfs sandbox cgroup: %v", err)
			} else {
				// we need to remove this defer and delete the cgroup once conmon exits
				// maybe need a conmon monitor?
				if err := control.Add(cgroups.Process{Pid: cmd.Process.Pid}); err != nil {
					logrus.Warnf("Failed to add conmon to cgroupfs sandbox cgroup: %v", err)
				}
			}
		}
	}
	return nil
}
