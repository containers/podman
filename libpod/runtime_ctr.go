package libpod

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/common/pkg/config"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/libpod/events"
	"github.com/containers/libpod/pkg/cgroups"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/storage/pkg/stringid"
	"github.com/docker/go-units"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Contains the public Runtime API for containers

// A CtrCreateOption is a functional option which alters the Container created
// by NewContainer
type CtrCreateOption func(*Container) error

// ContainerFilter is a function to determine whether a container is included
// in command output. Containers to be outputted are tested using the function.
// A true return will include the container, a false return will exclude it.
type ContainerFilter func(*Container) bool

// NewContainer creates a new container from a given OCI config.
func (r *Runtime) NewContainer(ctx context.Context, rSpec *spec.Spec, options ...CtrCreateOption) (c *Container, err error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	if !r.valid {
		return nil, define.ErrRuntimeStopped
	}
	return r.newContainer(ctx, rSpec, options...)
}

// RestoreContainer re-creates a container from an imported checkpoint
func (r *Runtime) RestoreContainer(ctx context.Context, rSpec *spec.Spec, config *ContainerConfig) (c *Container, err error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	if !r.valid {
		return nil, define.ErrRuntimeStopped
	}

	ctr, err := r.initContainerVariables(rSpec, config)
	if err != nil {
		return nil, errors.Wrapf(err, "error initializing container variables")
	}
	// For an imported checkpoint no one has ever set the StartedTime. Set it now.
	ctr.state.StartedTime = time.Now()

	// If the path to ConmonPidFile starts with the default value (RunRoot), then
	// the user has not specified '--conmon-pidfile' during run or create (probably).
	// In that case reset ConmonPidFile to be set to the default value later.
	if strings.HasPrefix(ctr.config.ConmonPidFile, r.storageConfig.RunRoot) {
		ctr.config.ConmonPidFile = ""
	}

	return r.setupContainer(ctx, ctr)
}

func (r *Runtime) initContainerVariables(rSpec *spec.Spec, config *ContainerConfig) (c *Container, err error) {
	if rSpec == nil {
		return nil, errors.Wrapf(define.ErrInvalidArg, "must provide a valid runtime spec to create container")
	}
	ctr := new(Container)
	ctr.config = new(ContainerConfig)
	ctr.state = new(ContainerState)

	if config == nil {
		ctr.config.ID = stringid.GenerateNonCryptoID()
		size, err := units.FromHumanSize(r.config.Containers.ShmSize)
		if err != nil {
			return nil, errors.Wrapf(err, "converting containers.conf ShmSize %s to an int", r.config.Containers.ShmSize)
		}
		ctr.config.ShmSize = size
	} else {
		// This is a restore from an imported checkpoint
		ctr.restoreFromCheckpoint = true
		if err := JSONDeepCopy(config, ctr.config); err != nil {
			return nil, errors.Wrapf(err, "error copying container config for restore")
		}
		// If the ID is empty a new name for the restored container was requested
		if ctr.config.ID == "" {
			ctr.config.ID = stringid.GenerateNonCryptoID()
			// Fixup ExitCommand with new ID
			ctr.config.ExitCommand[len(ctr.config.ExitCommand)-1] = ctr.config.ID
		}
		// Reset the log path to point to the default
		ctr.config.LogPath = ""
	}

	ctr.config.Spec = new(spec.Spec)
	if err := JSONDeepCopy(rSpec, ctr.config.Spec); err != nil {
		return nil, errors.Wrapf(err, "error copying runtime spec while creating container")
	}
	ctr.config.CreatedTime = time.Now()

	ctr.state.BindMounts = make(map[string]string)

	ctr.config.StopTimeout = r.config.Engine.StopTimeout

	ctr.config.OCIRuntime = r.defaultOCIRuntime.Name()

	// Set namespace based on current runtime namespace
	// Do so before options run so they can override it
	if r.config.Engine.Namespace != "" {
		ctr.config.Namespace = r.config.Engine.Namespace
	}

	ctr.runtime = r

	return ctr, nil
}

func (r *Runtime) newContainer(ctx context.Context, rSpec *spec.Spec, options ...CtrCreateOption) (c *Container, err error) {
	span, _ := opentracing.StartSpanFromContext(ctx, "newContainer")
	span.SetTag("type", "runtime")
	defer span.Finish()

	ctr, err := r.initContainerVariables(rSpec, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "error initializing container variables")
	}

	for _, option := range options {
		if err := option(ctr); err != nil {
			return nil, errors.Wrapf(err, "error running container create option")
		}
	}

	return r.setupContainer(ctx, ctr)
}

func (r *Runtime) setupContainer(ctx context.Context, ctr *Container) (_ *Container, err error) {
	// Validate the container
	if err := ctr.validate(); err != nil {
		return nil, err
	}

	// Allocate a lock for the container
	lock, err := r.lockManager.AllocateLock()
	if err != nil {
		return nil, errors.Wrapf(err, "error allocating lock for new container")
	}
	ctr.lock = lock
	ctr.config.LockID = ctr.lock.ID()
	logrus.Debugf("Allocated lock %d for container %s", ctr.lock.ID(), ctr.ID())

	defer func() {
		if err != nil {
			if err2 := ctr.lock.Free(); err2 != nil {
				logrus.Errorf("Error freeing lock for container after creation failed: %v", err2)
			}
		}
	}()

	ctr.valid = true
	ctr.state.State = define.ContainerStateConfigured
	ctr.runtime = r

	if ctr.config.OCIRuntime == "" {
		ctr.ociRuntime = r.defaultOCIRuntime
	} else {
		ociRuntime, ok := r.ociRuntimes[ctr.config.OCIRuntime]
		if !ok {
			return nil, errors.Wrapf(define.ErrInvalidArg, "requested OCI runtime %s is not available", ctr.config.OCIRuntime)
		}
		ctr.ociRuntime = ociRuntime
	}

	// Check NoCgroups support
	if ctr.config.NoCgroups {
		if !ctr.ociRuntime.SupportsNoCgroups() {
			return nil, errors.Wrapf(define.ErrInvalidArg, "requested OCI runtime %s is not compatible with NoCgroups", ctr.ociRuntime.Name())
		}
	}

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

	// Check CGroup parent sanity, and set it if it was not set.
	// Only if we're actually configuring CGroups.
	if !ctr.config.NoCgroups {
		switch r.config.Engine.CgroupManager {
		case config.CgroupfsCgroupsManager:
			if ctr.config.CgroupParent == "" {
				if pod != nil && pod.config.UsePodCgroup {
					podCgroup, err := pod.CgroupPath()
					if err != nil {
						return nil, errors.Wrapf(err, "error retrieving pod %s cgroup", pod.ID())
					}
					if podCgroup == "" {
						return nil, errors.Wrapf(define.ErrInternal, "pod %s cgroup is not set", pod.ID())
					}
					ctr.config.CgroupParent = podCgroup
				} else {
					ctr.config.CgroupParent = CgroupfsDefaultCgroupParent
				}
			} else if strings.HasSuffix(path.Base(ctr.config.CgroupParent), ".slice") {
				return nil, errors.Wrapf(define.ErrInvalidArg, "systemd slice received as cgroup parent when using cgroupfs")
			}
		case config.SystemdCgroupsManager:
			if ctr.config.CgroupParent == "" {
				switch {
				case pod != nil && pod.config.UsePodCgroup:
					podCgroup, err := pod.CgroupPath()
					if err != nil {
						return nil, errors.Wrapf(err, "error retrieving pod %s cgroup", pod.ID())
					}
					ctr.config.CgroupParent = podCgroup
				case rootless.IsRootless():
					ctr.config.CgroupParent = SystemdDefaultRootlessCgroupParent
				default:
					ctr.config.CgroupParent = SystemdDefaultCgroupParent
				}
			} else if len(ctr.config.CgroupParent) < 6 || !strings.HasSuffix(path.Base(ctr.config.CgroupParent), ".slice") {
				return nil, errors.Wrapf(define.ErrInvalidArg, "did not receive systemd slice as cgroup parent when using systemd to manage cgroups")
			}
		default:
			return nil, errors.Wrapf(define.ErrInvalidArg, "unsupported CGroup manager: %s - cannot validate cgroup parent", r.config.Engine.CgroupManager)
		}
	}

	if ctr.restoreFromCheckpoint {
		// Remove information about bind mount
		// for new container from imported checkpoint
		g := generate.Generator{Config: ctr.config.Spec}
		g.RemoveMount("/dev/shm")
		ctr.config.ShmDir = ""
		g.RemoveMount("/etc/resolv.conf")
		g.RemoveMount("/etc/hostname")
		g.RemoveMount("/etc/hosts")
		g.RemoveMount("/run/.containerenv")
		g.RemoveMount("/run/secrets")

		// Regenerate CGroup paths so they don't point to the old
		// container ID.
		cgroupPath, err := ctr.getOCICgroupPath()
		if err != nil {
			return nil, err
		}
		g.SetLinuxCgroupsPath(cgroupPath)
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

	if ctr.config.ConmonPidFile == "" {
		ctr.config.ConmonPidFile = filepath.Join(ctr.state.RunDir, "conmon.pid")
	}

	// Go through named volumes and add them.
	// If they don't exist they will be created using basic options.
	// Maintain an array of them - we need to lock them later.
	ctrNamedVolumes := make([]*Volume, 0, len(ctr.config.NamedVolumes))
	for _, vol := range ctr.config.NamedVolumes {
		isAnonymous := false
		if vol.Name == "" {
			// Anonymous volume. We'll need to create it.
			// It needs a name first.
			vol.Name = stringid.GenerateNonCryptoID()
			isAnonymous = true
		} else {
			// Check if it exists already
			dbVol, err := r.state.Volume(vol.Name)
			if err == nil {
				ctrNamedVolumes = append(ctrNamedVolumes, dbVol)
				// The volume exists, we're good
				continue
			} else if errors.Cause(err) != define.ErrNoSuchVolume {
				return nil, errors.Wrapf(err, "error retrieving named volume %s for new container", vol.Name)
			}
		}

		logrus.Debugf("Creating new volume %s for container", vol.Name)

		// The volume does not exist, so we need to create it.
		volOptions := []VolumeCreateOption{WithVolumeName(vol.Name), WithVolumeUID(ctr.RootUID()), WithVolumeGID(ctr.RootGID())}
		if isAnonymous {
			volOptions = append(volOptions, withSetAnon())
		}
		newVol, err := r.newVolume(ctx, volOptions...)
		if err != nil {
			return nil, errors.Wrapf(err, "error creating named volume %q", vol.Name)
		}

		ctrNamedVolumes = append(ctrNamedVolumes, newVol)
	}

	if ctr.config.LogPath == "" && ctr.config.LogDriver != define.JournaldLogging {
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

	// Lock all named volumes we are adding ourself to, to ensure we can't
	// use a volume being removed.
	for _, namedVol := range ctrNamedVolumes {
		toLock := namedVol
		toLock.lock.Lock()
		defer toLock.lock.Unlock()
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
	} else if err := r.state.AddContainer(ctr); err != nil {
		return nil, err
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
	return r.removeContainer(ctx, c, force, removeVolume, false)
}

// Internal function to remove a container.
// Locks the container, but does not lock the runtime.
// removePod is used only when removing pods. It instructs Podman to ignore
// infra container protections, and *not* remove from the database (as pod
// remove will handle that).
func (r *Runtime) removeContainer(ctx context.Context, c *Container, force bool, removeVolume bool, removePod bool) error {
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

	logrus.Debugf("Removing container %s", c.ID())

	// We need to lock the pod before we lock the container.
	// To avoid races around removing a container and the pod it is in.
	// Don't need to do this in pod removal case - we're evicting the entire
	// pod.
	var pod *Pod
	var err error
	runtime := c.runtime
	if c.config.Pod != "" && !removePod {
		pod, err = r.state.Pod(c.config.Pod)
		if err != nil {
			return errors.Wrapf(err, "container %s is in pod %s, but pod cannot be retrieved", c.ID(), pod.ID())
		}

		// Lock the pod while we're removing container
		if pod.config.LockID == c.config.LockID {
			return errors.Wrapf(define.ErrWillDeadlock, "container %s and pod %s share lock ID %d", c.ID(), pod.ID(), c.config.LockID)
		}
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

	// For pod removal, the container is already locked by the caller
	if !removePod {
		c.lock.Lock()
		defer c.lock.Unlock()
	}

	if !r.valid {
		return define.ErrRuntimeStopped
	}

	// Update the container to get current state
	if err := c.syncContainer(); err != nil {
		return err
	}

	// If we're not force-removing, we need to check if we're in a good
	// state to remove.
	if !force {
		if err := c.checkReadyForRemoval(); err != nil {
			return err
		}
	}

	if c.state.State == define.ContainerStatePaused {
		if err := c.ociRuntime.KillContainer(c, 9, false); err != nil {
			return err
		}
		isV2, err := cgroups.IsCgroup2UnifiedMode()
		if err != nil {
			return err
		}
		// cgroups v1 and v2 handle signals on paused processes differently
		if !isV2 {
			if err := c.unpause(); err != nil {
				return err
			}
		}
		// Need to update container state to make sure we know it's stopped
		if err := c.waitForExitFileAndSync(); err != nil {
			return err
		}
	}

	// Check that the container's in a good state to be removed
	if c.state.State == define.ContainerStateRunning {
		if err := c.stop(c.StopTimeout()); err != nil {
			return errors.Wrapf(err, "cannot remove container %s as it could not be stopped", c.ID())
		}
	}

	// Remove all active exec sessions
	if err := c.removeAllExecSessions(); err != nil {
		return err
	}

	// Check that no other containers depend on the container.
	// Only used if not removing a pod - pods guarantee that all
	// deps will be evicted at the same time.
	if !removePod {
		deps, err := r.state.ContainerInUse(c)
		if err != nil {
			return err
		}
		if len(deps) != 0 {
			depsStr := strings.Join(deps, ", ")
			return errors.Wrapf(define.ErrCtrExists, "container %s has dependent containers which must be removed before it: %s", c.ID(), depsStr)
		}
	}

	var cleanupErr error

	// Clean up network namespace, cgroups, mounts.
	// Do this before we set ContainerStateRemoving, to ensure that we can
	// actually remove from the OCI runtime.
	if err := c.cleanup(ctx); err != nil {
		cleanupErr = errors.Wrapf(err, "error cleaning up container %s", c.ID())
	}

	// Set ContainerStateRemoving
	c.state.State = define.ContainerStateRemoving

	if err := c.save(); err != nil {
		if cleanupErr != nil {
			logrus.Errorf(err.Error())
		}
		return errors.Wrapf(err, "unable to set container %s removing state in database", c.ID())
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
	if c.state.State != define.ContainerStateConfigured &&
		c.state.State != define.ContainerStateExited {
		if err := c.delete(ctx); err != nil {
			if cleanupErr == nil {
				cleanupErr = err
			} else {
				logrus.Errorf("delete container: %v", err)
			}
		}
	}

	// Remove the container from the state
	if c.config.Pod != "" {
		// If we're removing the pod, the container will be evicted
		// from the state elsewhere
		if !removePod {
			if err := r.state.RemoveContainerFromPod(pod, c); err != nil {
				if cleanupErr == nil {
					cleanupErr = err
				} else {
					logrus.Errorf("Error removing container %s from database: %v", c.ID(), err)
				}
			}
		}
	} else {
		if err := r.state.RemoveContainer(c); err != nil {
			if cleanupErr == nil {
				cleanupErr = err
			} else {
				logrus.Errorf("Error removing container %s from database: %v", c.ID(), err)
			}
		}
	}

	// Deallocate the container's lock
	if err := c.lock.Free(); err != nil {
		if cleanupErr == nil {
			cleanupErr = errors.Wrapf(err, "error freeing lock for container %s", c.ID())
		} else {
			logrus.Errorf("free container lock: %v", err)
		}
	}

	// Set container as invalid so it can no longer be used
	c.valid = false

	c.newContainerEvent(events.Remove)

	if !removeVolume {
		return cleanupErr
	}

	for _, v := range c.config.NamedVolumes {
		if volume, err := runtime.state.Volume(v.Name); err == nil {
			if !volume.Anonymous() {
				continue
			}
			if err := runtime.removeVolume(ctx, volume, false); err != nil && errors.Cause(err) != define.ErrNoSuchVolume {
				logrus.Errorf("cleanup volume (%s): %v", v, err)
			}
		}
	}

	return cleanupErr
}

// EvictContainer removes the given container partial or full ID or name, and
// returns the full ID of the evicted container and any error encountered.
// It should be used to remove a container when obtaining a Container struct
// pointer has failed.
// Running container will not be stopped.
// If removeVolume is specified, named volumes used by the container will
// be removed also if and only if the container is the sole user.
func (r *Runtime) EvictContainer(ctx context.Context, idOrName string, removeVolume bool) (string, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return r.evictContainer(ctx, idOrName, removeVolume)
}

// evictContainer is the internal function to handle container eviction based
// on its partial or full ID or name.
// It returns the full ID of the evicted container and any error encountered.
// This does not lock the runtime nor the container.
// removePod is used only when removing pods. It instructs Podman to ignore
// infra container protections, and *not* remove from the database (as pod
// remove will handle that).
func (r *Runtime) evictContainer(ctx context.Context, idOrName string, removeVolume bool) (string, error) {
	var err error

	if !r.valid {
		return "", define.ErrRuntimeStopped
	}

	id, err := r.state.LookupContainerID(idOrName)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to find container %q in state", idOrName)
	}

	// Begin by trying a normal removal. Valid containers will be removed normally.
	tmpCtr, err := r.state.Container(id)
	if err == nil {
		logrus.Infof("Container %s successfully retrieved from state, attempting normal removal", id)
		// Assume force = true for the evict case
		err = r.removeContainer(ctx, tmpCtr, true, removeVolume, false)
		if !tmpCtr.valid {
			// If the container is marked invalid, remove succeeded
			// in kicking it out of the state - no need to continue.
			return id, err
		}

		if err == nil {
			// Something has gone seriously wrong - no error but
			// container was not removed.
			logrus.Errorf("Container %s not removed with no error", id)
		} else {
			logrus.Warnf("Failed to removal container %s normally, proceeding with evict: %v", id, err)
		}
	}

	// Error out if the container does not exist in libpod
	exists, err := r.state.HasContainer(id)
	if err != nil {
		return id, err
	}
	if !exists {
		return id, errors.Wrapf(err, "Failed to find container ID %q for eviction", id)
	}

	// Re-create a container struct for removal purposes
	c := new(Container)
	c.config, err = r.state.GetContainerConfig(id)
	if err != nil {
		return id, errors.Wrapf(err, "failed to retrieve config for ctr ID %q", id)
	}
	c.state = new(ContainerState)

	// We need to lock the pod before we lock the container.
	// To avoid races around removing a container and the pod it is in.
	// Don't need to do this in pod removal case - we're evicting the entire
	// pod.
	var pod *Pod
	if c.config.Pod != "" {
		pod, err = r.state.Pod(c.config.Pod)
		if err != nil {
			return id, errors.Wrapf(err, "container %s is in pod %s, but pod cannot be retrieved", c.ID(), pod.ID())
		}

		// Lock the pod while we're removing container
		pod.lock.Lock()
		defer pod.lock.Unlock()
		if err := pod.updatePod(); err != nil {
			return id, err
		}

		infraID := pod.state.InfraContainerID
		if c.ID() == infraID {
			return id, errors.Errorf("container %s is the infra container of pod %s and cannot be removed without removing the pod", c.ID(), pod.ID())
		}
	}

	var cleanupErr error
	// Remove the container from the state
	if c.config.Pod != "" {
		// If we're removing the pod, the container will be evicted
		// from the state elsewhere
		if err := r.state.RemoveContainerFromPod(pod, c); err != nil {
			cleanupErr = err
		}
	} else {
		if err := r.state.RemoveContainer(c); err != nil {
			cleanupErr = err
		}
	}

	// Unmount container mount points
	for _, mount := range c.config.Mounts {
		Unmount(mount)
	}

	// Remove container from c/storage
	if err := r.removeStorageContainer(id, true); err != nil {
		if cleanupErr == nil {
			cleanupErr = err
		}
	}

	if !removeVolume {
		return id, cleanupErr
	}

	for _, v := range c.config.NamedVolumes {
		if volume, err := r.state.Volume(v.Name); err == nil {
			if !volume.Anonymous() {
				continue
			}
			if err := r.removeVolume(ctx, volume, false); err != nil && err != define.ErrNoSuchVolume && err != define.ErrVolumeBeingUsed {
				logrus.Errorf("cleanup volume (%s): %v", v, err)
			}
		}
	}

	return id, cleanupErr
}

// GetContainer retrieves a container by its ID
func (r *Runtime) GetContainer(id string) (*Container, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, define.ErrRuntimeStopped
	}

	return r.state.Container(id)
}

// HasContainer checks if a container with the given ID is present
func (r *Runtime) HasContainer(id string) (bool, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return false, define.ErrRuntimeStopped
	}

	return r.state.HasContainer(id)
}

// LookupContainer looks up a container by its name or a partial ID
// If a partial ID is not unique, an error will be returned
func (r *Runtime) LookupContainer(idOrName string) (*Container, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, define.ErrRuntimeStopped
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
		return nil, define.ErrRuntimeStopped
	}

	ctrs, err := r.GetAllContainers()
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
		return state == define.ContainerStateRunning
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
		return nil, define.ErrNoSuchCtr
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

// GetExecSessionContainer gets the container that a given exec session ID is
// attached to.
func (r *Runtime) GetExecSessionContainer(id string) (*Container, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, define.ErrRuntimeStopped
	}

	ctrID, err := r.state.GetExecSession(id)
	if err != nil {
		return nil, err
	}

	return r.state.Container(ctrID)
}

// PruneContainers removes stopped and exited containers from localstorage.  A set of optional filters
// can be provided to be more granular.
func (r *Runtime) PruneContainers(filterFuncs []ContainerFilter) (map[string]int64, map[string]error, error) {
	pruneErrors := make(map[string]error)
	prunedContainers := make(map[string]int64)
	// We add getting the exited and stopped containers via a filter
	containerStateFilter := func(c *Container) bool {
		if c.PodID() != "" {
			return false
		}
		state, err := c.State()
		if err != nil {
			logrus.Error(err)
			return false
		}
		if state == define.ContainerStateStopped || state == define.ContainerStateExited ||
			state == define.ContainerStateCreated || state == define.ContainerStateConfigured {
			return true
		}
		return false
	}
	filterFuncs = append(filterFuncs, containerStateFilter)
	delContainers, err := r.GetContainers(filterFuncs...)
	if err != nil {
		return nil, nil, err
	}
	for _, c := range delContainers {
		ctr := c
		size, err := ctr.RWSize()
		if err != nil {
			pruneErrors[ctr.ID()] = err
			continue
		}
		err = r.RemoveContainer(context.Background(), ctr, false, false)
		if err != nil {
			pruneErrors[ctr.ID()] = err
		} else {
			prunedContainers[ctr.ID()] = size
		}
	}
	return prunedContainers, pruneErrors, nil
}
