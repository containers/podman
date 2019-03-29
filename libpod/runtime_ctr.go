package libpod

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/containers/libpod/libpod/events"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/stringid"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// CtrRemoveTimeout is the default number of seconds to wait after stopping a container
// before sending the kill signal
const CtrRemoveTimeout = 10

// Contains the public Runtime API for containers

// A CtrCreateOption is a functional option which alters the Container created
// by NewContainer
type CtrCreateOption func(*Container) error

// ContainerFilter is a function to determine whether a container is included
// in command output. Containers to be outputted are tested using the function.
// A true return will include the container, a false return will exclude it.
type ContainerFilter func(*Container) bool

// NewContainer creates a new container from a given OCI config
func (r *Runtime) NewContainer(ctx context.Context, rSpec *spec.Spec, options ...CtrCreateOption) (c *Container, err error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	if !r.valid {
		return nil, ErrRuntimeStopped
	}
	return r.newContainer(ctx, rSpec, options...)
}

func (r *Runtime) newContainer(ctx context.Context, rSpec *spec.Spec, options ...CtrCreateOption) (c *Container, err error) {
	span, _ := opentracing.StartSpanFromContext(ctx, "newContainer")
	span.SetTag("type", "runtime")
	defer span.Finish()

	if rSpec == nil {
		return nil, errors.Wrapf(ErrInvalidArg, "must provide a valid runtime spec to create container")
	}

	ctr := new(Container)
	ctr.config = new(ContainerConfig)
	ctr.state = new(ContainerState)

	ctr.config.ID = stringid.GenerateNonCryptoID()

	ctr.config.Spec = new(spec.Spec)
	if err := JSONDeepCopy(rSpec, ctr.config.Spec); err != nil {
		return nil, errors.Wrapf(err, "error copying runtime spec while creating container")
	}
	ctr.config.CreatedTime = time.Now()

	ctr.config.ShmSize = DefaultShmSize

	ctr.state.BindMounts = make(map[string]string)

	ctr.config.StopTimeout = CtrRemoveTimeout

	ctr.config.OCIRuntime = r.config.OCIRuntime

	// Set namespace based on current runtime namespace
	// Do so before options run so they can override it
	if r.config.Namespace != "" {
		ctr.config.Namespace = r.config.Namespace
	}

	ctr.runtime = r
	for _, option := range options {
		if err := option(ctr); err != nil {
			return nil, errors.Wrapf(err, "error running container create option")
		}
	}

	// Allocate a lock for the container
	lock, err := r.lockManager.AllocateLock()
	if err != nil {
		return nil, errors.Wrapf(err, "error allocating lock for new container")
	}
	ctr.lock = lock
	ctr.config.LockID = ctr.lock.ID()
	logrus.Debugf("Allocated lock %d for container %s", ctr.lock.ID(), ctr.ID())

	ctr.valid = true
	ctr.state.State = ContainerStateConfigured
	ctr.runtime = r

	ctr.valid = true
	ctr.state.State = ContainerStateConfigured

	var pod *Pod
	if ctr.config.Pod != "" {
		// Get the pod from state
		pod, err = r.state.Pod(ctr.config.Pod)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot add container %s to pod %s", ctr.ID(), ctr.config.Pod)
		}
	}

	if ctr.config.Name == "" {
		name, err := r.generateName()
		if err != nil {
			return nil, err
		}

		ctr.config.Name = name
	}

	// Check CGroup parent sanity, and set it if it was not set
	switch r.config.CgroupManager {
	case CgroupfsCgroupsManager:
		if ctr.config.CgroupParent == "" {
			if pod != nil && pod.config.UsePodCgroup {
				podCgroup, err := pod.CgroupPath()
				if err != nil {
					return nil, errors.Wrapf(err, "error retrieving pod %s cgroup", pod.ID())
				}
				if podCgroup == "" {
					return nil, errors.Wrapf(ErrInternal, "pod %s cgroup is not set", pod.ID())
				}
				ctr.config.CgroupParent = podCgroup
			} else {
				ctr.config.CgroupParent = CgroupfsDefaultCgroupParent
			}
		} else if strings.HasSuffix(path.Base(ctr.config.CgroupParent), ".slice") {
			return nil, errors.Wrapf(ErrInvalidArg, "systemd slice received as cgroup parent when using cgroupfs")
		}
	case SystemdCgroupsManager:
		if ctr.config.CgroupParent == "" {
			if pod != nil && pod.config.UsePodCgroup {
				podCgroup, err := pod.CgroupPath()
				if err != nil {
					return nil, errors.Wrapf(err, "error retrieving pod %s cgroup", pod.ID())
				}
				ctr.config.CgroupParent = podCgroup
			} else {
				ctr.config.CgroupParent = SystemdDefaultCgroupParent
			}
		} else if len(ctr.config.CgroupParent) < 6 || !strings.HasSuffix(path.Base(ctr.config.CgroupParent), ".slice") {
			return nil, errors.Wrapf(ErrInvalidArg, "did not receive systemd slice as cgroup parent when using systemd to manage cgroups")
		}
	default:
		return nil, errors.Wrapf(ErrInvalidArg, "unsupported CGroup manager: %s - cannot validate cgroup parent", r.config.CgroupManager)
	}

	// Set up storage for the container
	if err := ctr.setupStorage(ctx); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			if err2 := ctr.teardownStorage(); err2 != nil {
				logrus.Errorf("Error removing partially-created container root filesystem: %s", err2)
			}
		}
	}()

	if rootless.IsRootless() && ctr.config.ConmonPidFile == "" {
		ctr.config.ConmonPidFile = filepath.Join(ctr.config.StaticDir, "conmon.pid")
	}

	// Go through the volume mounts and check for named volumes
	// If the named volme already exists continue, otherwise create
	// the storage for the named volume.
	for i, vol := range ctr.config.Spec.Mounts {
		if vol.Source[0] != '/' && isNamedVolume(vol.Source) {
			volInfo, err := r.state.Volume(vol.Source)
			if err != nil {
				newVol, err := r.newVolume(ctx, WithVolumeName(vol.Source), withSetCtrSpecific(), WithVolumeUID(ctr.RootUID()), WithVolumeGID(ctr.RootGID()))
				if err != nil {
					return nil, errors.Wrapf(err, "error creating named volume %q", vol.Source)
				}
				ctr.config.Spec.Mounts[i].Source = newVol.MountPoint()
				if err := ctr.copyWithTarFromImage(ctr.config.Spec.Mounts[i].Destination, ctr.config.Spec.Mounts[i].Source); err != nil && !os.IsNotExist(err) {
					return nil, errors.Wrapf(err, "failed to copy content into new volume mount %q", vol.Source)
				}
				continue
			}
			ctr.config.Spec.Mounts[i].Source = volInfo.MountPoint()
		}
	}

	if ctr.config.LogPath == "" {
		ctr.config.LogPath = filepath.Join(ctr.config.StaticDir, "ctr.log")
	}

	if !MountExists(ctr.config.Spec.Mounts, "/dev/shm") && ctr.config.ShmDir == "" {
		ctr.config.ShmDir = filepath.Join(ctr.bundlePath(), "shm")
		if err := os.MkdirAll(ctr.config.ShmDir, 0700); err != nil {
			if !os.IsExist(err) {
				return nil, errors.Wrapf(err, "unable to create shm %q dir", ctr.config.ShmDir)
			}
		}
		ctr.config.Mounts = append(ctr.config.Mounts, ctr.config.ShmDir)
	}

	// Add the container to the state
	// TODO: May be worth looking into recovering from name/ID collisions here
	if ctr.config.Pod != "" {
		// Lock the pod to ensure we can't add containers to pods
		// being removed
		pod.lock.Lock()
		defer pod.lock.Unlock()

		if err := r.state.AddContainerToPod(pod, ctr); err != nil {
			return nil, err
		}
	} else {
		if err := r.state.AddContainer(ctr); err != nil {
			return nil, err
		}
	}
	ctr.newContainerEvent(events.Create)
	return ctr, nil
}

// RemoveContainer removes the given container
// If force is specified, the container will be stopped first
// If removeVolume is specified, named volumes used by the container will
// be removed also if and only if the container is the sole user
// Otherwise, RemoveContainer will return an error if the container is running
func (r *Runtime) RemoveContainer(ctx context.Context, c *Container, force bool, removeVolume bool) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.removeContainer(ctx, c, force, removeVolume)
}

// Internal function to remove a container
// Locks the container, but does not lock the runtime
func (r *Runtime) removeContainer(ctx context.Context, c *Container, force bool, removeVolume bool) error {
	span, _ := opentracing.StartSpanFromContext(ctx, "removeContainer")
	span.SetTag("type", "runtime")
	defer span.Finish()

	if !c.valid {
		if ok, _ := r.state.HasContainer(c.ID()); !ok {
			// Container probably already removed
			// Or was never in the runtime to begin with
			return nil
		}
	}

	// We need to lock the pod before we lock the container
	// To avoid races around removing a container and the pod it is in
	var pod *Pod
	var err error
	runtime := c.runtime
	if c.config.Pod != "" {
		pod, err = r.state.Pod(c.config.Pod)
		if err != nil {
			return errors.Wrapf(err, "container %s is in pod %s, but pod cannot be retrieved", c.ID(), pod.ID())
		}

		// Lock the pod while we're removing container
		pod.lock.Lock()
		defer pod.lock.Unlock()
		if err := pod.updatePod(); err != nil {
			return err
		}

		infraID := pod.state.InfraContainerID
		if c.ID() == infraID {
			return errors.Errorf("container %s is the infra container of pod %s and cannot be removed without removing the pod", c.ID(), pod.ID())
		}
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	if !r.valid {
		return ErrRuntimeStopped
	}

	// Update the container to get current state
	if err := c.syncContainer(); err != nil {
		return err
	}

	if c.state.State == ContainerStatePaused {
		if !force {
			return errors.Wrapf(ErrCtrStateInvalid, "container %s is paused, cannot remove until unpaused", c.ID())
		}
		if err := c.runtime.ociRuntime.killContainer(c, 9); err != nil {
			return err
		}
		if err := c.unpause(); err != nil {
			return err
		}
		// Need to update container state to make sure we know it's stopped
		if err := c.waitForExitFileAndSync(); err != nil {
			return err
		}
	}

	// Check that the container's in a good state to be removed
	if c.state.State == ContainerStateRunning && force {
		if err := r.ociRuntime.stopContainer(c, c.StopTimeout()); err != nil {
			return errors.Wrapf(err, "cannot remove container %s as it could not be stopped", c.ID())
		}

		// Need to update container state to make sure we know it's stopped
		if err := c.waitForExitFileAndSync(); err != nil {
			return err
		}
	} else if !(c.state.State == ContainerStateConfigured ||
		c.state.State == ContainerStateCreated ||
		c.state.State == ContainerStateStopped ||
		c.state.State == ContainerStateExited) {
		return errors.Wrapf(ErrCtrStateInvalid, "cannot remove container %s as it is %s - running or paused containers cannot be removed", c.ID(), c.state.State.String())
	}

	// Check that all of our exec sessions have finished
	if len(c.state.ExecSessions) != 0 {
		if force {
			if err := r.ociRuntime.execStopContainer(c, c.StopTimeout()); err != nil {
				return err
			}
		} else {
			return errors.Wrapf(ErrCtrStateInvalid, "cannot remove container %s as it has active exec sessions", c.ID())
		}
	}

	// Check that no other containers depend on the container
	deps, err := r.state.ContainerInUse(c)
	if err != nil {
		return err
	}
	if len(deps) != 0 {
		depsStr := strings.Join(deps, ", ")
		return errors.Wrapf(ErrCtrExists, "container %s has dependent containers which must be removed before it: %s", c.ID(), depsStr)
	}

	var volumes []string
	if removeVolume {
		volumes, err = c.namedVolumes()
		if err != nil {
			logrus.Errorf("unable to retrieve builtin volumes for container %v: %v", c.ID(), err)
		}
	}
	var cleanupErr error
	// Remove the container from the state
	if c.config.Pod != "" {
		if err := r.state.RemoveContainerFromPod(pod, c); err != nil {
			if cleanupErr == nil {
				cleanupErr = err
			} else {
				logrus.Errorf("removing container from pod: %v", err)
			}
		}
	} else {
		if err := r.state.RemoveContainer(c); err != nil {
			if cleanupErr == nil {
				cleanupErr = err
			} else {
				logrus.Errorf("removing container: %v", err)
			}
		}
	}

	// Set container as invalid so it can no longer be used
	c.valid = false

	// Clean up network namespace, cgroups, mounts
	if err := c.cleanup(ctx); err != nil {
		if cleanupErr == nil {
			cleanupErr = err
		} else {
			logrus.Errorf("cleanup network, cgroups, mounts: %v", err)
		}
	}

	// Stop the container's storage
	if err := c.teardownStorage(); err != nil {
		if cleanupErr == nil {
			cleanupErr = err
		} else {
			logrus.Errorf("cleanup storage: %v", err)
		}
	}

	// Delete the container.
	// Not needed in Configured and Exited states, where the container
	// doesn't exist in the runtime
	if c.state.State != ContainerStateConfigured &&
		c.state.State != ContainerStateExited {
		if err := c.delete(ctx); err != nil {
			if cleanupErr == nil {
				cleanupErr = err
			} else {
				logrus.Errorf("delete container: %v", err)
			}
		}
	}

	// Deallocate the container's lock
	if err := c.lock.Free(); err != nil {
		if cleanupErr == nil {
			cleanupErr = err
		} else {
			logrus.Errorf("free container lock: %v", err)
		}
	}

	for _, v := range volumes {
		if volume, err := runtime.state.Volume(v); err == nil {
			if !volume.IsCtrSpecific() {
				continue
			}
			if err := runtime.removeVolume(ctx, volume, false); err != nil && err != ErrNoSuchVolume && err != ErrVolumeBeingUsed {
				logrus.Errorf("cleanup volume (%s): %v", v, err)
			}
		}
	}

	c.newContainerEvent(events.Remove)
	return cleanupErr
}

// GetContainer retrieves a container by its ID
func (r *Runtime) GetContainer(id string) (*Container, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, ErrRuntimeStopped
	}

	return r.state.Container(id)
}

// HasContainer checks if a container with the given ID is present
func (r *Runtime) HasContainer(id string) (bool, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return false, ErrRuntimeStopped
	}

	return r.state.HasContainer(id)
}

// LookupContainer looks up a container by its name or a partial ID
// If a partial ID is not unique, an error will be returned
func (r *Runtime) LookupContainer(idOrName string) (*Container, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, ErrRuntimeStopped
	}
	return r.state.LookupContainer(idOrName)
}

// GetContainers retrieves all containers from the state
// Filters can be provided which will determine what containers are included in
// the output. Multiple filters are handled by ANDing their output, so only
// containers matching all filters are returned
func (r *Runtime) GetContainers(filters ...ContainerFilter) ([]*Container, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, ErrRuntimeStopped
	}

	ctrs, err := r.state.AllContainers()
	if err != nil {
		return nil, err
	}

	ctrsFiltered := make([]*Container, 0, len(ctrs))

	for _, ctr := range ctrs {
		include := true
		for _, filter := range filters {
			include = include && filter(ctr)
		}

		if include {
			ctrsFiltered = append(ctrsFiltered, ctr)
		}
	}

	return ctrsFiltered, nil
}

// GetAllContainers is a helper function for GetContainers
func (r *Runtime) GetAllContainers() ([]*Container, error) {
	return r.state.AllContainers()
}

// GetRunningContainers is a helper function for GetContainers
func (r *Runtime) GetRunningContainers() ([]*Container, error) {
	running := func(c *Container) bool {
		state, _ := c.State()
		return state == ContainerStateRunning
	}
	return r.GetContainers(running)
}

// GetContainersByList is a helper function for GetContainers
// which takes a []string of container IDs or names
func (r *Runtime) GetContainersByList(containers []string) ([]*Container, error) {
	var ctrs []*Container
	for _, inputContainer := range containers {
		ctr, err := r.LookupContainer(inputContainer)
		if err != nil {
			return ctrs, errors.Wrapf(err, "unable to lookup container %s", inputContainer)
		}
		ctrs = append(ctrs, ctr)
	}
	return ctrs, nil
}

// GetLatestContainer returns a container object of the latest created container.
func (r *Runtime) GetLatestContainer() (*Container, error) {
	lastCreatedIndex := -1
	var lastCreatedTime time.Time
	ctrs, err := r.GetAllContainers()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to find latest container")
	}
	if len(ctrs) == 0 {
		return nil, ErrNoSuchCtr
	}
	for containerIndex, ctr := range ctrs {
		createdTime := ctr.config.CreatedTime
		if createdTime.After(lastCreatedTime) {
			lastCreatedTime = createdTime
			lastCreatedIndex = containerIndex
		}
	}
	return ctrs[lastCreatedIndex], nil
}

// Check if volName is a named volume and not one of the default mounts we add to containers
func isNamedVolume(volName string) bool {
	if volName != "proc" && volName != "tmpfs" && volName != "devpts" && volName != "shm" && volName != "mqueue" && volName != "sysfs" && volName != "cgroup" {
		return true
	}
	return false
}

// Export is the libpod portion of exporting a container to a tar file
func (r *Runtime) Export(name string, path string) error {
	ctr, err := r.LookupContainer(name)
	if err != nil {
		return err
	}
	if os.Geteuid() != 0 {
		state, err := ctr.State()
		if err != nil {
			return errors.Wrapf(err, "cannot read container state %q", ctr.ID())
		}
		if state == ContainerStateRunning || state == ContainerStatePaused {
			data, err := ioutil.ReadFile(ctr.Config().ConmonPidFile)
			if err != nil {
				return errors.Wrapf(err, "cannot read conmon PID file %q", ctr.Config().ConmonPidFile)
			}
			conmonPid, err := strconv.Atoi(string(data))
			if err != nil {
				return errors.Wrapf(err, "cannot parse PID %q", data)
			}
			became, ret, err := rootless.JoinDirectUserAndMountNS(uint(conmonPid))
			if err != nil {
				return err
			}
			if became {
				os.Exit(ret)
			}
		} else {
			became, ret, err := rootless.BecomeRootInUserNS()
			if err != nil {
				return err
			}
			if became {
				os.Exit(ret)
			}
		}
	}
	return ctr.Export(path)

}

// RemoveContainersFromStorage attempt to remove containers from storage that do not exist in libpod database
func (r *Runtime) RemoveContainersFromStorage(ctrs []string) {
	for _, i := range ctrs {
		// if the container does not exist in database, attempt to remove it from storage
		if _, err := r.LookupContainer(i); err != nil && errors.Cause(err) == image.ErrNoSuchCtr {
			r.storageService.UnmountContainerImage(i, true)
			if err := r.storageService.DeleteContainer(i); err != nil && errors.Cause(err) != storage.ErrContainerUnknown {
				logrus.Errorf("Failed to remove container %q from storage: %s", i, err)
			}
		}
	}
}
