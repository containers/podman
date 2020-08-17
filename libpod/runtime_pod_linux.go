// +build linux

package libpod

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/libpod/events"
	"github.com/containers/podman/v2/pkg/cgroups"
	"github.com/containers/podman/v2/pkg/rootless"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// NewPod makes a new, empty pod
func (r *Runtime) NewPod(ctx context.Context, options ...PodCreateOption) (_ *Pod, deferredErr error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.valid {
		return nil, define.ErrRuntimeStopped
	}

	pod := newPod(r)

	// Set default namespace to runtime's namespace
	// Do so before options run so they can override it
	if r.config.Engine.Namespace != "" {
		pod.config.Namespace = r.config.Engine.Namespace
	}

	for _, option := range options {
		if err := option(pod); err != nil {
			return nil, errors.Wrapf(err, "error running pod create option")
		}
	}

	if pod.config.Name == "" {
		name, err := r.generateName()
		if err != nil {
			return nil, err
		}
		pod.config.Name = name
	}

	if pod.config.Hostname == "" {
		pod.config.Hostname = pod.config.Name
	}

	// Allocate a lock for the pod
	lock, err := r.lockManager.AllocateLock()
	if err != nil {
		return nil, errors.Wrapf(err, "error allocating lock for new pod")
	}
	pod.lock = lock
	pod.config.LockID = pod.lock.ID()

	defer func() {
		if deferredErr != nil {
			if err := pod.lock.Free(); err != nil {
				logrus.Errorf("Error freeing pod lock after failed creation: %v", err)
			}
		}
	}()

	pod.valid = true

	// Check CGroup parent sanity, and set it if it was not set
	switch r.config.Engine.CgroupManager {
	case config.CgroupfsCgroupsManager:
		if pod.config.CgroupParent == "" {
			pod.config.CgroupParent = CgroupfsDefaultCgroupParent
		} else if strings.HasSuffix(path.Base(pod.config.CgroupParent), ".slice") {
			return nil, errors.Wrapf(define.ErrInvalidArg, "systemd slice received as cgroup parent when using cgroupfs")
		}
		// If we are set to use pod cgroups, set the cgroup parent that
		// all containers in the pod will share
		// No need to create it with cgroupfs - the first container to
		// launch should do it for us
		if pod.config.UsePodCgroup {
			pod.state.CgroupPath = filepath.Join(pod.config.CgroupParent, pod.ID())
		}
	case config.SystemdCgroupsManager:
		if pod.config.CgroupParent == "" {
			if rootless.IsRootless() {
				pod.config.CgroupParent = SystemdDefaultRootlessCgroupParent
			} else {
				pod.config.CgroupParent = SystemdDefaultCgroupParent
			}
		} else if len(pod.config.CgroupParent) < 6 || !strings.HasSuffix(path.Base(pod.config.CgroupParent), ".slice") {
			return nil, errors.Wrapf(define.ErrInvalidArg, "did not receive systemd slice as cgroup parent when using systemd to manage cgroups")
		}
		// If we are set to use pod cgroups, set the cgroup parent that
		// all containers in the pod will share
		if pod.config.UsePodCgroup {
			cgroupPath, err := systemdSliceFromPath(pod.config.CgroupParent, fmt.Sprintf("libpod_pod_%s", pod.ID()))
			if err != nil {
				return nil, errors.Wrapf(err, "unable to create pod cgroup for pod %s", pod.ID())
			}
			pod.state.CgroupPath = cgroupPath
		}
	default:
		return nil, errors.Wrapf(define.ErrInvalidArg, "unsupported CGroup manager: %s - cannot validate cgroup parent", r.config.Engine.CgroupManager)
	}

	if pod.config.UsePodCgroup {
		logrus.Debugf("Got pod cgroup as %s", pod.state.CgroupPath)
	}
	if !pod.HasInfraContainer() && pod.SharesNamespaces() {
		return nil, errors.Errorf("Pods must have an infra container to share namespaces")
	}
	if pod.HasInfraContainer() && !pod.SharesNamespaces() {
		logrus.Warnf("Pod has an infra container, but shares no namespaces")
	}

	if err := r.state.AddPod(pod); err != nil {
		return nil, errors.Wrapf(err, "error adding pod to state")
	}
	defer func() {
		if deferredErr != nil {
			if err := r.removePod(ctx, pod, true, true); err != nil {
				logrus.Errorf("Error removing pod after pause container creation failure: %v", err)
			}
		}
	}()

	if pod.HasInfraContainer() {
		ctr, err := r.createInfraContainer(ctx, pod)
		if err != nil {
			return nil, errors.Wrapf(err, "error adding Infra Container")
		}
		pod.state.InfraContainerID = ctr.ID()
		if err := pod.save(); err != nil {
			return nil, err
		}
	}
	pod.newPodEvent(events.Create)
	return pod, nil
}

func (r *Runtime) removePod(ctx context.Context, p *Pod, removeCtrs, force bool) error {
	if err := p.updatePod(); err != nil {
		return err
	}

	ctrs, err := r.state.PodContainers(p)
	if err != nil {
		return err
	}

	numCtrs := len(ctrs)

	// If the only container in the pod is the pause container, remove the pod and container unconditionally.
	pauseCtrID := p.state.InfraContainerID
	if numCtrs == 1 && ctrs[0].ID() == pauseCtrID {
		removeCtrs = true
		force = true
	}
	if !removeCtrs && numCtrs > 0 {
		return errors.Wrapf(define.ErrCtrExists, "pod %s contains containers and cannot be removed", p.ID())
	}

	// Go through and lock all containers so we can operate on them all at
	// once.
	// First loop also checks that we are ready to go ahead and remove.
	for _, ctr := range ctrs {
		ctrLock := ctr.lock
		ctrLock.Lock()
		defer ctrLock.Unlock()

		// If we're force-removing, no need to check status.
		if force {
			continue
		}

		// Sync all containers
		if err := ctr.syncContainer(); err != nil {
			return err
		}

		// Ensure state appropriate for removal
		if err := ctr.checkReadyForRemoval(); err != nil {
			return errors.Wrapf(err, "pod %s has containers that are not ready to be removed", p.ID())
		}
	}

	// We're going to be removing containers.
	// If we are CGroupfs cgroup driver, to avoid races, we need to hit
	// the pod and conmon CGroups with a PID limit to prevent them from
	// spawning any further processes (particularly cleanup processes) which
	// would prevent removing the CGroups.
	if p.runtime.config.Engine.CgroupManager == config.CgroupfsCgroupsManager {
		// Get the conmon CGroup
		conmonCgroupPath := filepath.Join(p.state.CgroupPath, "conmon")
		conmonCgroup, err := cgroups.Load(conmonCgroupPath)
		if err != nil && err != cgroups.ErrCgroupDeleted && err != cgroups.ErrCgroupV1Rootless {
			logrus.Errorf("Error retrieving pod %s conmon cgroup %s: %v", p.ID(), conmonCgroupPath, err)
		}

		// New resource limits
		resLimits := new(spec.LinuxResources)
		resLimits.Pids = new(spec.LinuxPids)
		resLimits.Pids.Limit = 1 // Inhibit forks with very low pids limit

		// Don't try if we failed to retrieve the cgroup
		if err == nil {
			if err := conmonCgroup.Update(resLimits); err != nil {
				logrus.Warnf("Error updating pod %s conmon cgroup %s PID limit: %v", p.ID(), conmonCgroupPath, err)
			}
		}
	}

	var removalErr error

	ctrNamedVolumes := make(map[string]*ContainerNamedVolume)

	// Second loop - all containers are good, so we should be clear to
	// remove.
	for _, ctr := range ctrs {
		// Remove the container.
		// Do NOT remove named volumes. Instead, we're going to build a
		// list of them to be removed at the end, once the containers
		// have been removed by RemovePodContainers.
		for _, vol := range ctr.config.NamedVolumes {
			ctrNamedVolumes[vol.Name] = vol
		}

		if err := r.removeContainer(ctx, ctr, force, false, true); err != nil {
			if removalErr == nil {
				removalErr = err
			} else {
				logrus.Errorf("Error removing container %s from pod %s: %v", ctr.ID(), p.ID(), err)
			}
		}
	}

	// Remove all containers in the pod from the state.
	if err := r.state.RemovePodContainers(p); err != nil {
		// If this fails, there isn't much more we can do.
		// The containers in the pod are unusable, but they still exist,
		// so pod removal will fail.
		return err
	}

	for volName := range ctrNamedVolumes {
		volume, err := r.state.Volume(volName)
		if err != nil && errors.Cause(err) != define.ErrNoSuchVolume {
			logrus.Errorf("Error retrieving volume %s: %v", volName, err)
			continue
		}
		if !volume.Anonymous() {
			continue
		}
		if err := r.removeVolume(ctx, volume, false); err != nil {
			if errors.Cause(err) == define.ErrNoSuchVolume || errors.Cause(err) == define.ErrVolumeRemoved {
				continue
			}
			logrus.Errorf("Error removing volume %s: %v", volName, err)
		}
	}

	// Remove pod cgroup, if present
	if p.state.CgroupPath != "" {
		logrus.Debugf("Removing pod cgroup %s", p.state.CgroupPath)

		switch p.runtime.config.Engine.CgroupManager {
		case config.SystemdCgroupsManager:
			if err := deleteSystemdCgroup(p.state.CgroupPath); err != nil {
				if removalErr == nil {
					removalErr = errors.Wrapf(err, "error removing pod %s cgroup", p.ID())
				} else {
					logrus.Errorf("Error deleting pod %s cgroup %s: %v", p.ID(), p.state.CgroupPath, err)
				}
			}
		case config.CgroupfsCgroupsManager:
			// Delete the cgroupfs cgroup
			// Make sure the conmon cgroup is deleted first
			// Since the pod is almost gone, don't bother failing
			// hard - instead, just log errors.
			conmonCgroupPath := filepath.Join(p.state.CgroupPath, "conmon")
			conmonCgroup, err := cgroups.Load(conmonCgroupPath)
			if err != nil && err != cgroups.ErrCgroupDeleted && err != cgroups.ErrCgroupV1Rootless {
				if removalErr == nil {
					removalErr = errors.Wrapf(err, "error retrieving pod %s conmon cgroup", p.ID())
				} else {
					logrus.Debugf("Error retrieving pod %s conmon cgroup %s: %v", p.ID(), conmonCgroupPath, err)
				}
			}
			if err == nil {
				if err := conmonCgroup.Delete(); err != nil {
					if removalErr == nil {
						removalErr = errors.Wrapf(err, "error removing pod %s conmon cgroup", p.ID())
					} else {
						logrus.Errorf("Error deleting pod %s conmon cgroup %s: %v", p.ID(), conmonCgroupPath, err)
					}
				}
			}
			cgroup, err := cgroups.Load(p.state.CgroupPath)
			if err != nil && err != cgroups.ErrCgroupDeleted && err != cgroups.ErrCgroupV1Rootless {
				if removalErr == nil {
					removalErr = errors.Wrapf(err, "error retrieving pod %s cgroup", p.ID())
				} else {
					logrus.Errorf("Error retrieving pod %s cgroup %s: %v", p.ID(), p.state.CgroupPath, err)
				}
			}
			if err == nil {
				if err := cgroup.Delete(); err != nil {
					if removalErr == nil {
						removalErr = errors.Wrapf(err, "error removing pod %s cgroup", p.ID())
					} else {
						logrus.Errorf("Error deleting pod %s cgroup %s: %v", p.ID(), p.state.CgroupPath, err)
					}
				}
			}
		default:
			// This should be caught much earlier, but let's still
			// keep going so we make sure to evict the pod before
			// ending up with an inconsistent state.
			if removalErr == nil {
				removalErr = errors.Wrapf(define.ErrInternal, "unrecognized cgroup manager %s when removing pod %s cgroups", p.runtime.config.Engine.CgroupManager, p.ID())
			} else {
				logrus.Errorf("Unknown cgroups manager %s specified - cannot remove pod %s cgroup", p.runtime.config.Engine.CgroupManager, p.ID())
			}
		}
	}

	// Remove pod from state
	if err := r.state.RemovePod(p); err != nil {
		if removalErr != nil {
			logrus.Errorf("%v", removalErr)
		}
		return err
	}

	// Mark pod invalid
	p.valid = false
	p.newPodEvent(events.Remove)

	// Deallocate the pod lock
	if err := p.lock.Free(); err != nil {
		if removalErr == nil {
			removalErr = errors.Wrapf(err, "error freeing pod %s lock", p.ID())
		} else {
			logrus.Errorf("Error freeing pod %s lock: %v", p.ID(), err)
		}
	}

	return removalErr
}
