package libpod

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/storage/pkg/stringid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Creates a new, empty pod
func newPod(runtime *Runtime) *Pod {
	pod := new(Pod)
	pod.config = new(PodConfig)
	pod.config.ID = stringid.GenerateNonCryptoID()
	pod.config.Labels = make(map[string]string)
	pod.config.CreatedTime = time.Now()
	//	pod.config.InfraContainer = new(ContainerConfig)
	pod.state = new(podState)
	pod.runtime = runtime

	return pod
}

// Update pod state from database
func (p *Pod) updatePod() error {
	if err := p.runtime.state.UpdatePod(p); err != nil {
		return err
	}

	return nil
}

// Save pod state to database
func (p *Pod) save() error {
	if err := p.runtime.state.SavePod(p); err != nil {
		return errors.Wrapf(err, "error saving pod %s state", p.ID())
	}

	return nil
}

// Refresh a pod's state after restart
// This cannot lock any other pod, but may lock individual containers, as those
// will have refreshed by the time pod refresh runs.
func (p *Pod) refresh() error {
	// Need to to an update from the DB to pull potentially-missing state
	if err := p.runtime.state.UpdatePod(p); err != nil {
		return err
	}

	if !p.valid {
		return define.ErrPodRemoved
	}

	// Retrieve the pod's lock
	lock, err := p.runtime.lockManager.AllocateAndRetrieveLock(p.config.LockID)
	if err != nil {
		return errors.Wrapf(err, "error retrieving lock %d for pod %s", p.config.LockID, p.ID())
	}
	p.lock = lock

	// We need to recreate the pod's cgroup
	if p.config.UsePodCgroup {
		switch p.runtime.config.Engine.CgroupManager {
		case config.SystemdCgroupsManager:
			cgroupPath, err := systemdSliceFromPath(p.config.CgroupParent, fmt.Sprintf("libpod_pod_%s", p.ID()))
			if err != nil {
				logrus.Errorf("Creating Cgroup for pod %s: %v", p.ID(), err)
			}
			p.state.CgroupPath = cgroupPath
		case config.CgroupfsCgroupsManager:
			if rootless.IsRootless() && isRootlessCgroupSet(p.config.CgroupParent) {
				p.state.CgroupPath = filepath.Join(p.config.CgroupParent, p.ID())

				logrus.Debugf("setting pod cgroup to %s", p.state.CgroupPath)
			}
		default:
			return errors.Wrapf(define.ErrInvalidArg, "unknown cgroups manager %s specified", p.runtime.config.Engine.CgroupManager)
		}
	}

	// Save changes
	return p.save()
}
