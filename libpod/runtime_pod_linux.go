package libpod

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/sirupsen/logrus"
)

func (r *Runtime) platformMakePod(pod *Pod, p specgen.PodSpecGenerator) error {
	// Check Cgroup parent sanity, and set it if it was not set
	if r.config.Cgroups() != "disabled" {
		switch r.config.Engine.CgroupManager {
		case config.CgroupfsCgroupsManager:
			canUseCgroup := !rootless.IsRootless() || isRootlessCgroupSet(pod.config.CgroupParent)
			if canUseCgroup {
				// need to actually create parent here
				if pod.config.CgroupParent == "" {
					pod.config.CgroupParent = CgroupfsDefaultCgroupParent
				} else if strings.HasSuffix(path.Base(pod.config.CgroupParent), ".slice") {
					return fmt.Errorf("systemd slice received as cgroup parent when using cgroupfs: %w", define.ErrInvalidArg)
				}
				// If we are set to use pod cgroups, set the cgroup parent that
				// all containers in the pod will share
				if pod.config.UsePodCgroup {
					pod.state.CgroupPath = filepath.Join(pod.config.CgroupParent, pod.ID())
					if p.InfraContainerSpec != nil {
						p.InfraContainerSpec.CgroupParent = pod.state.CgroupPath
						// cgroupfs + rootless = permission denied when creating the cgroup.
						if !rootless.IsRootless() {
							res, err := GetLimits(p.ResourceLimits)
							if err != nil {
								return err
							}
							// Need to both create and update the cgroup
							// rather than create a new path in c/common for pod cgroup creation
							// just create as if it is a ctr and then update figures out that we need to
							// populate the resource limits on the pod level
							cgc, err := cgroups.New(pod.state.CgroupPath, &res)
							if err != nil {
								return err
							}
							err = cgc.Update(&res)
							if err != nil {
								return err
							}
						}
					}
				}
			}
		case config.SystemdCgroupsManager:
			if pod.config.CgroupParent == "" {
				if rootless.IsRootless() {
					pod.config.CgroupParent = SystemdDefaultRootlessCgroupParent
				} else {
					pod.config.CgroupParent = SystemdDefaultCgroupParent
				}
			} else if len(pod.config.CgroupParent) < 6 || !strings.HasSuffix(path.Base(pod.config.CgroupParent), ".slice") {
				return fmt.Errorf("did not receive systemd slice as cgroup parent when using systemd to manage cgroups: %w", define.ErrInvalidArg)
			}
			// If we are set to use pod cgroups, set the cgroup parent that
			// all containers in the pod will share
			if pod.config.UsePodCgroup {
				cgroupPath, err := systemdSliceFromPath(pod.config.CgroupParent, fmt.Sprintf("libpod_pod_%s", pod.ID()), p.ResourceLimits)
				if err != nil {
					return fmt.Errorf("unable to create pod cgroup for pod %s: %w", pod.ID(), err)
				}
				pod.state.CgroupPath = cgroupPath
				if p.InfraContainerSpec != nil {
					p.InfraContainerSpec.CgroupParent = pod.state.CgroupPath
				}
			}
		default:
			return fmt.Errorf("unsupported Cgroup manager: %s - cannot validate cgroup parent: %w", r.config.Engine.CgroupManager, define.ErrInvalidArg)
		}
	}

	if pod.config.UsePodCgroup {
		logrus.Debugf("Got pod cgroup as %s", pod.state.CgroupPath)
	}

	return nil
}
