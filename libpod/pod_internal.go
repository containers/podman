package libpod

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/storage"
	"github.com/containers/storage/pkg/stringid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Creates a new, empty pod
func newPod(lockDir string, runtime *Runtime) (*Pod, error) {
	pod := new(Pod)
	pod.config = new(PodConfig)
	pod.config.ID = stringid.GenerateNonCryptoID()
	pod.config.Labels = make(map[string]string)
	pod.config.CreatedTime = time.Now()
	pod.config.PauseContainer = new(PauseContainerConfig)
	pod.state = new(podState)
	pod.runtime = runtime

	// Path our lock file will reside at
	lockPath := filepath.Join(lockDir, pod.config.ID)
	// Grab a lockfile at the given path
	lock, err := storage.GetLockfile(lockPath)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating lockfile for new pod")
	}
	pod.lock = lock

	return pod, nil
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
		return errors.Wrapf(err, "error saving pod %s state")
	}

	return nil
}

// Refresh a pod's state after restart
func (p *Pod) refresh() error {
	// Need to to an update from the DB to pull potentially-missing state
	if err := p.runtime.state.UpdatePod(p); err != nil {
		return err
	}

	if !p.valid {
		return ErrPodRemoved
	}

	// We need to recreate the pod's cgroup
	if p.config.UsePodCgroup {
		switch p.runtime.config.CgroupManager {
		case SystemdCgroupsManager:
			cgroupPath, err := systemdSliceFromPath(p.config.CgroupParent, fmt.Sprintf("libpod_pod_%s", p.ID()))
			if err != nil {
				logrus.Errorf("Error creating CGroup for pod %s: %v", p.ID(), err)
			}
			p.state.CgroupPath = cgroupPath
		case CgroupfsCgroupsManager:
			p.state.CgroupPath = filepath.Join(p.config.CgroupParent, p.ID())

			logrus.Debugf("setting pod cgroup to %s", p.state.CgroupPath)
		default:
			return errors.Wrapf(ErrInvalidArg, "unknown cgroups manager %s specified", p.runtime.config.CgroupManager)
		}
	}

	// Save changes
	return p.save()
}

// Visit a node on a container graph and start the container, or set an error if
// a dependency failed to start. if restart is true, startNode will restart the node instead of starting it.
func startNode(ctx context.Context, node *containerNode, setError bool, ctrErrors map[string]error, ctrsVisited map[string]bool, restart bool) {
	// First, check if we have already visited the node
	if ctrsVisited[node.id] {
		return
	}

	// If setError is true, a dependency of us failed
	// Mark us as failed and recurse
	if setError {
		// Mark us as visited, and set an error
		ctrsVisited[node.id] = true
		ctrErrors[node.id] = errors.Wrapf(ErrCtrStateInvalid, "a dependency of container %s failed to start", node.id)

		// Hit anyone who depends on us, and set errors on them too
		for _, successor := range node.dependedOn {
			startNode(ctx, successor, true, ctrErrors, ctrsVisited, restart)
		}

		return
	}

	// Have all our dependencies started?
	// If not, don't visit the node yet
	depsVisited := true
	for _, dep := range node.dependsOn {
		depsVisited = depsVisited && ctrsVisited[dep.id]
	}
	if !depsVisited {
		// Don't visit us yet, all dependencies are not up
		// We'll hit the dependencies eventually, and when we do it will
		// recurse here
		return
	}

	// Going to try to start the container, mark us as visited
	ctrsVisited[node.id] = true

	ctrErrored := false

	// Check if dependencies are running
	// Graph traversal means we should have started them
	// But they could have died before we got here
	// Does not require that the container be locked, we only need to lock
	// the dependencies
	depsStopped, err := node.container.checkDependenciesRunning()
	if err != nil {
		ctrErrors[node.id] = err
		ctrErrored = true
	} else if len(depsStopped) > 0 {
		// Our dependencies are not running
		depsList := strings.Join(depsStopped, ",")
		ctrErrors[node.id] = errors.Wrapf(ErrCtrStateInvalid, "the following dependencies of container %s are not running: %s", node.id, depsList)
		ctrErrored = true
	}

	// Lock before we start
	node.container.lock.Lock()

	// Sync the container to pick up current state
	if !ctrErrored {
		if err := node.container.syncContainer(); err != nil {
			ctrErrored = true
			ctrErrors[node.id] = err
		}
	}

	// Start the container (only if it is not running)
	if !ctrErrored {
		if !restart && node.container.state.State != ContainerStateRunning {
			if err := node.container.initAndStart(ctx); err != nil {
				ctrErrored = true
				ctrErrors[node.id] = err
			}
		}
		if restart && node.container.state.State != ContainerStatePaused && node.container.state.State != ContainerStateUnknown {
			if err := node.container.restartWithTimeout(ctx, node.container.config.StopTimeout); err != nil {
				ctrErrored = true
				ctrErrors[node.id] = err
			}
		}
	}

	node.container.lock.Unlock()

	// Recurse to anyone who depends on us and start them
	for _, successor := range node.dependedOn {
		startNode(ctx, successor, ctrErrored, ctrErrors, ctrsVisited, restart)
	}

	return
}
