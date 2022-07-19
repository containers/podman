//go:build linux
// +build linux

package libpod

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/sirupsen/logrus"
)

// NewPod makes a new, empty pod
func (r *Runtime) NewPod(ctx context.Context, p specgen.PodSpecGenerator, options ...PodCreateOption) (_ *Pod, deferredErr error) {
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
			return nil, fmt.Errorf("error running pod create option: %w", err)
		}
	}

	// Allocate a lock for the pod
	lock, err := r.lockManager.AllocateLock()
	if err != nil {
		return nil, fmt.Errorf("error allocating lock for new pod: %w", err)
	}
	pod.lock = lock
	pod.config.LockID = pod.lock.ID()

	defer func() {
		if deferredErr != nil {
			if err := pod.lock.Free(); err != nil {
				logrus.Errorf("Freeing pod lock after failed creation: %v", err)
			}
		}
	}()

	pod.valid = true

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
					return nil, fmt.Errorf("systemd slice received as cgroup parent when using cgroupfs: %w", define.ErrInvalidArg)
				}
				// If we are set to use pod cgroups, set the cgroup parent that
				// all containers in the pod will share
				if pod.config.UsePodCgroup {
					pod.state.CgroupPath = filepath.Join(pod.config.CgroupParent, pod.ID())
					if p.InfraContainerSpec != nil {
						p.InfraContainerSpec.CgroupParent = pod.state.CgroupPath
						// cgroupfs + rootless = permission denied when creating the cgroup.
						if !rootless.IsRootless() {
							res, err := GetLimits(p.InfraContainerSpec.ResourceLimits)
							if err != nil {
								return nil, err
							}
							// Need to both create and update the cgroup
							// rather than create a new path in c/common for pod cgroup creation
							// just create as if it is a ctr and then update figures out that we need to
							// populate the resource limits on the pod level
							cgc, err := cgroups.New(pod.state.CgroupPath, &res)
							if err != nil {
								return nil, err
							}
							err = cgc.Update(&res)
							if err != nil {
								return nil, err
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
				return nil, fmt.Errorf("did not receive systemd slice as cgroup parent when using systemd to manage cgroups: %w", define.ErrInvalidArg)
			}
			// If we are set to use pod cgroups, set the cgroup parent that
			// all containers in the pod will share
			if pod.config.UsePodCgroup {
				cgroupPath, err := systemdSliceFromPath(pod.config.CgroupParent, fmt.Sprintf("libpod_pod_%s", pod.ID()), p.InfraContainerSpec.ResourceLimits)
				if err != nil {
					return nil, fmt.Errorf("unable to create pod cgroup for pod %s: %w", pod.ID(), err)
				}
				pod.state.CgroupPath = cgroupPath
				if p.InfraContainerSpec != nil {
					p.InfraContainerSpec.CgroupParent = pod.state.CgroupPath
				}
			}
		default:
			return nil, fmt.Errorf("unsupported Cgroup manager: %s - cannot validate cgroup parent: %w", r.config.Engine.CgroupManager, define.ErrInvalidArg)
		}
	}

	if pod.config.UsePodCgroup {
		logrus.Debugf("Got pod cgroup as %s", pod.state.CgroupPath)
	}

	if !pod.HasInfraContainer() && pod.SharesNamespaces() {
		return nil, errors.New("Pods must have an infra container to share namespaces")
	}
	if pod.HasInfraContainer() && !pod.SharesNamespaces() {
		logrus.Infof("Pod has an infra container, but shares no namespaces")
	}

	// Unless the user has specified a name, use a randomly generated one.
	// Note that name conflicts may occur (see #11735), so we need to loop.
	generateName := pod.config.Name == ""
	var addPodErr error
	for {
		if generateName {
			name, err := r.generateName()
			if err != nil {
				return nil, err
			}
			pod.config.Name = name
		}

		if p.InfraContainerSpec != nil && p.InfraContainerSpec.Hostname == "" {
			p.InfraContainerSpec.Hostname = pod.config.Name
		}
		if addPodErr = r.state.AddPod(pod); addPodErr == nil {
			return pod, nil
		}
		if !generateName || (!errors.Is(addPodErr, define.ErrPodExists) && !errors.Is(addPodErr, define.ErrCtrExists)) {
			break
		}
	}
	if addPodErr != nil {
		return nil, fmt.Errorf("error adding pod to state: %w", addPodErr)
	}

	return pod, nil
}

// AddInfra adds the created infra container to the pod state
func (r *Runtime) AddInfra(ctx context.Context, pod *Pod, infraCtr *Container) (*Pod, error) {
	if !r.valid {
		return nil, define.ErrRuntimeStopped
	}
	pod.state.InfraContainerID = infraCtr.ID()
	if err := pod.save(); err != nil {
		return nil, err
	}
	pod.newPodEvent(events.Create)
	return pod, nil
}

// SavePod is a helper function to save the pod state from outside of libpod
func (r *Runtime) SavePod(pod *Pod) error {
	if !r.valid {
		return define.ErrRuntimeStopped
	}
	if err := pod.save(); err != nil {
		return err
	}
	pod.newPodEvent(events.Create)
	return nil
}

func (r *Runtime) removePod(ctx context.Context, p *Pod, removeCtrs, force bool, timeout *uint) error {
	if err := p.updatePod(); err != nil {
		return err
	}

	ctrs, err := r.state.PodContainers(p)
	if err != nil {
		return err
	}
	numCtrs := len(ctrs)

	// If the only running container in the pod is the pause container, remove the pod and container unconditionally.
	pauseCtrID := p.state.InfraContainerID
	if numCtrs == 1 && ctrs[0].ID() == pauseCtrID {
		removeCtrs = true
		force = true
	}
	if !removeCtrs && numCtrs > 0 {
		return fmt.Errorf("pod %s contains containers and cannot be removed: %w", p.ID(), define.ErrCtrExists)
	}

	ctrNamedVolumes := make(map[string]*ContainerNamedVolume)

	var removalErr error
	for _, ctr := range ctrs {
		err := func() error {
			ctrLock := ctr.lock
			ctrLock.Lock()
			defer func() {
				ctrLock.Unlock()
			}()

			if err := ctr.syncContainer(); err != nil {
				return err
			}

			for _, vol := range ctr.config.NamedVolumes {
				ctrNamedVolumes[vol.Name] = vol
			}

			return r.removeContainer(ctx, ctr, force, false, true, timeout)
		}()

		if removalErr == nil {
			removalErr = err
		} else {
			logrus.Errorf("Removing container %s from pod %s: %v", ctr.ID(), p.ID(), err)
		}
	}
	if removalErr != nil {
		return removalErr
	}

	// Clear infra container ID before we remove the infra container.
	// There is a potential issue if we don't do that, and removal is
	// interrupted between RemoveAllContainers() below and the pod's removal
	// later - we end up with a reference to a nonexistent infra container.
	p.state.InfraContainerID = ""
	if err := p.save(); err != nil {
		return err
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
		if err != nil && !errors.Is(err, define.ErrNoSuchVolume) {
			logrus.Errorf("Retrieving volume %s: %v", volName, err)
			continue
		}
		if !volume.Anonymous() {
			continue
		}
		if err := r.removeVolume(ctx, volume, false, timeout, false); err != nil {
			if errors.Is(err, define.ErrNoSuchVolume) || errors.Is(err, define.ErrVolumeRemoved) {
				continue
			}
			logrus.Errorf("Removing volume %s: %v", volName, err)
		}
	}

	// Remove pod cgroup, if present
	if p.state.CgroupPath != "" {
		logrus.Debugf("Removing pod cgroup %s", p.state.CgroupPath)

		switch p.runtime.config.Engine.CgroupManager {
		case config.SystemdCgroupsManager:
			if err := deleteSystemdCgroup(p.state.CgroupPath, p.ResourceLim()); err != nil {
				if removalErr == nil {
					removalErr = fmt.Errorf("error removing pod %s cgroup: %w", p.ID(), err)
				} else {
					logrus.Errorf("Deleting pod %s cgroup %s: %v", p.ID(), p.state.CgroupPath, err)
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
					removalErr = fmt.Errorf("error retrieving pod %s conmon cgroup: %w", p.ID(), err)
				} else {
					logrus.Debugf("Error retrieving pod %s conmon cgroup %s: %v", p.ID(), conmonCgroupPath, err)
				}
			}
			if err == nil {
				if err = conmonCgroup.Delete(); err != nil {
					if removalErr == nil {
						removalErr = fmt.Errorf("error removing pod %s conmon cgroup: %w", p.ID(), err)
					} else {
						logrus.Errorf("Deleting pod %s conmon cgroup %s: %v", p.ID(), conmonCgroupPath, err)
					}
				}
			}
			cgroup, err := cgroups.Load(p.state.CgroupPath)
			if err != nil && err != cgroups.ErrCgroupDeleted && err != cgroups.ErrCgroupV1Rootless {
				if removalErr == nil {
					removalErr = fmt.Errorf("error retrieving pod %s cgroup: %w", p.ID(), err)
				} else {
					logrus.Errorf("Retrieving pod %s cgroup %s: %v", p.ID(), p.state.CgroupPath, err)
				}
			}
			if err == nil {
				if err := cgroup.Delete(); err != nil {
					if removalErr == nil {
						removalErr = fmt.Errorf("error removing pod %s cgroup: %w", p.ID(), err)
					} else {
						logrus.Errorf("Deleting pod %s cgroup %s: %v", p.ID(), p.state.CgroupPath, err)
					}
				}
			}
		default:
			// This should be caught much earlier, but let's still
			// keep going so we make sure to evict the pod before
			// ending up with an inconsistent state.
			if removalErr == nil {
				removalErr = fmt.Errorf("unrecognized cgroup manager %s when removing pod %s cgroups: %w", p.runtime.config.Engine.CgroupManager, p.ID(), define.ErrInternal)
			} else {
				logrus.Errorf("Unknown cgroups manager %s specified - cannot remove pod %s cgroup", p.runtime.config.Engine.CgroupManager, p.ID())
			}
		}
	}

	if err := p.maybeRemoveServiceContainer(); err != nil {
		return err
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
			removalErr = fmt.Errorf("error freeing pod %s lock: %w", p.ID(), err)
		} else {
			logrus.Errorf("Freeing pod %s lock: %v", p.ID(), err)
		}
	}

	return removalErr
}
