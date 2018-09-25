package libpod

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/containers/libpod/pkg/chrootuser"
	"github.com/containers/libpod/pkg/hooks"
	"github.com/containers/libpod/pkg/hooks/exec"
	"github.com/containers/libpod/pkg/resolvconf"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/secrets"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/chrootarchive"
	"github.com/containers/storage/pkg/mount"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"
)

const (
	// name of the directory holding the artifacts
	artifactsDir = "artifacts"
)

var (
	// localeToLanguage maps from locale values to language tags.
	localeToLanguage = map[string]string{
		"":      "und-u-va-posix",
		"c":     "und-u-va-posix",
		"posix": "und-u-va-posix",
	}
)

// rootFsSize gets the size of the container's root filesystem
// A container FS is split into two parts.  The first is the top layer, a
// mutable layer, and the rest is the RootFS: the set of immutable layers
// that make up the image on which the container is based.
func (c *Container) rootFsSize() (int64, error) {
	if c.config.Rootfs != "" {
		return 0, nil
	}

	container, err := c.runtime.store.Container(c.ID())
	if err != nil {
		return 0, err
	}

	// Ignore the size of the top layer.   The top layer is a mutable RW layer
	// and is not considered a part of the rootfs
	rwLayer, err := c.runtime.store.Layer(container.LayerID)
	if err != nil {
		return 0, err
	}
	layer, err := c.runtime.store.Layer(rwLayer.Parent)
	if err != nil {
		return 0, err
	}

	size := int64(0)
	for layer.Parent != "" {
		layerSize, err := c.runtime.store.DiffSize(layer.Parent, layer.ID)
		if err != nil {
			return size, errors.Wrapf(err, "getting diffsize of layer %q and its parent %q", layer.ID, layer.Parent)
		}
		size += layerSize
		layer, err = c.runtime.store.Layer(layer.Parent)
		if err != nil {
			return 0, err
		}
	}
	// Get the size of the last layer.  Has to be outside of the loop
	// because the parent of the last layer is "", and lstore.Get("")
	// will return an error.
	layerSize, err := c.runtime.store.DiffSize(layer.Parent, layer.ID)
	return size + layerSize, err
}

// rwSize Gets the size of the mutable top layer of the container.
func (c *Container) rwSize() (int64, error) {
	if c.config.Rootfs != "" {
		var size int64
		err := filepath.Walk(c.config.Rootfs, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			size += info.Size()
			return nil
		})
		return size, err
	}

	container, err := c.runtime.store.Container(c.ID())
	if err != nil {
		return 0, err
	}

	// Get the size of the top layer by calculating the size of the diff
	// between the layer and its parent.  The top layer of a container is
	// the only RW layer, all others are immutable
	layer, err := c.runtime.store.Layer(container.LayerID)
	if err != nil {
		return 0, err
	}
	return c.runtime.store.DiffSize(layer.Parent, layer.ID)
}

// bundlePath returns the path to the container's root filesystem - where the OCI spec will be
// placed, amongst other things
func (c *Container) bundlePath() string {
	return c.config.StaticDir
}

// ControlSocketPath returns the path to the containers control socket for things like tty
// resizing
func (c *Container) ControlSocketPath() string {
	return filepath.Join(c.bundlePath(), "ctl")
}

// CheckpointPath returns the path to the directory containing the checkpoint
func (c *Container) CheckpointPath() string {
	return filepath.Join(c.bundlePath(), "checkpoint")
}

// AttachSocketPath retrieves the path of the container's attach socket
func (c *Container) AttachSocketPath() string {
	return filepath.Join(c.runtime.ociRuntime.socketsDir, c.ID(), "attach")
}

// Get PID file path for a container's exec session
func (c *Container) execPidPath(sessionID string) string {
	return filepath.Join(c.state.RunDir, "exec_pid_"+sessionID)
}

// Sync this container with on-disk state and runtime status
// Should only be called with container lock held
// This function should suffice to ensure a container's state is accurate and
// it is valid for use.
func (c *Container) syncContainer() error {
	if err := c.runtime.state.UpdateContainer(c); err != nil {
		return err
	}
	// If runtime knows about the container, update its status in runtime
	// And then save back to disk
	if (c.state.State != ContainerStateUnknown) &&
		(c.state.State != ContainerStateConfigured) &&
		(c.state.State != ContainerStateExited) {
		oldState := c.state.State
		// TODO: optionally replace this with a stat for the exit file
		if err := c.runtime.ociRuntime.updateContainerStatus(c); err != nil {
			return err
		}
		// Only save back to DB if state changed
		if c.state.State != oldState {
			if err := c.save(); err != nil {
				return err
			}
		}
	}

	if !c.valid {
		return errors.Wrapf(ErrCtrRemoved, "container %s is not valid", c.ID())
	}

	return nil
}

// Create container root filesystem for use
func (c *Container) setupStorage(ctx context.Context) error {
	if !c.valid {
		return errors.Wrapf(ErrCtrRemoved, "container %s is not valid", c.ID())
	}

	if c.state.State != ContainerStateConfigured {
		return errors.Wrapf(ErrCtrStateInvalid, "container %s must be in Configured state to have storage set up", c.ID())
	}

	// Need both an image ID and image name, plus a bool telling us whether to use the image configuration
	if c.config.Rootfs == "" && (c.config.RootfsImageID == "" || c.config.RootfsImageName == "") {
		return errors.Wrapf(ErrInvalidArg, "must provide image ID and image name to use an image")
	}

	var options *storage.ContainerOptions
	if c.config.Rootfs == "" {
		options = &storage.ContainerOptions{c.config.IDMappings}

	}
	containerInfo, err := c.runtime.storageService.CreateContainerStorage(ctx, c.runtime.imageContext, c.config.RootfsImageName, c.config.RootfsImageID, c.config.Name, c.config.ID, c.config.MountLabel, options)
	if err != nil {
		return errors.Wrapf(err, "error creating container storage")
	}

	if !rootless.IsRootless() && (len(c.config.IDMappings.UIDMap) != 0 || len(c.config.IDMappings.GIDMap) != 0) {
		info, err := os.Stat(c.runtime.config.TmpDir)
		if err != nil {
			return errors.Wrapf(err, "cannot stat `%s`", c.runtime.config.TmpDir)
		}
		if err := os.Chmod(c.runtime.config.TmpDir, info.Mode()|0111); err != nil {
			return errors.Wrapf(err, "cannot chmod `%s`", c.runtime.config.TmpDir)
		}
		root := filepath.Join(c.runtime.config.TmpDir, "containers-root", c.ID())
		if err := os.MkdirAll(root, 0755); err != nil {
			return errors.Wrapf(err, "error creating userNS tmpdir for container %s", c.ID())
		}
		if err := os.Chown(root, c.RootUID(), c.RootGID()); err != nil {
			return err
		}
		c.state.UserNSRoot, err = filepath.EvalSymlinks(root)
		if err != nil {
			return errors.Wrapf(err, "failed to eval symlinks for %s", root)
		}
	}

	c.config.StaticDir = containerInfo.Dir
	c.state.RunDir = containerInfo.RunDir
	c.state.DestinationRunDir = c.state.RunDir
	if c.state.UserNSRoot != "" {
		c.state.DestinationRunDir = filepath.Join(c.state.UserNSRoot, "rundir")
	}

	// Set the default Entrypoint and Command
	if c.config.Entrypoint == nil {
		c.config.Entrypoint = containerInfo.Config.Config.Entrypoint
	}
	if c.config.Command == nil {
		c.config.Command = containerInfo.Config.Config.Cmd
	}

	artifacts := filepath.Join(c.config.StaticDir, artifactsDir)
	if err := os.MkdirAll(artifacts, 0755); err != nil {
		return errors.Wrapf(err, "error creating artifacts directory %q", artifacts)
	}

	return nil
}

// Tear down a container's storage prior to removal
func (c *Container) teardownStorage() error {
	if c.state.State == ContainerStateRunning || c.state.State == ContainerStatePaused {
		return errors.Wrapf(ErrCtrStateInvalid, "cannot remove storage for container %s as it is running or paused", c.ID())
	}

	artifacts := filepath.Join(c.config.StaticDir, artifactsDir)
	if err := os.RemoveAll(artifacts); err != nil {
		return errors.Wrapf(err, "error removing artifacts %q", artifacts)
	}

	if err := c.cleanupStorage(); err != nil {
		return errors.Wrapf(err, "failed to cleanup container %s storage", c.ID())
	}

	if c.state.UserNSRoot != "" {
		if err := os.RemoveAll(c.state.UserNSRoot); err != nil {
			return errors.Wrapf(err, "error removing userns root %q", c.state.UserNSRoot)
		}
	}

	if err := c.runtime.storageService.DeleteContainer(c.ID()); err != nil {
		// If the container has already been removed, warn but do not
		// error - we wanted it gone, it is already gone.
		// Potentially another tool using containers/storage already
		// removed it?
		if err == storage.ErrNotAContainer || err == storage.ErrContainerUnknown {
			logrus.Warnf("Storage for container %s already removed", c.ID())
			return nil
		}

		return errors.Wrapf(err, "error removing container %s root filesystem", c.ID())
	}

	return nil
}

// Reset resets state fields to default values
// It is performed before a refresh and clears the state after a reboot
// It does not save the results - assumes the database will do that for us
func resetState(state *containerState) error {
	state.PID = 0
	state.Mountpoint = ""
	state.Mounted = false
	state.State = ContainerStateConfigured
	state.ExecSessions = make(map[string]*ExecSession)
	state.NetworkStatus = nil
	state.BindMounts = make(map[string]string)

	return nil
}

// Refresh refreshes the container's state after a restart
func (c *Container) refresh() error {
	// Don't need a full sync, but we do need to update from the database to
	// pick up potentially-missing container state
	if err := c.runtime.state.UpdateContainer(c); err != nil {
		return err
	}

	if !c.valid {
		return errors.Wrapf(ErrCtrRemoved, "container %s is not valid - may have been removed", c.ID())
	}

	// We need to get the container's temporary directory from c/storage
	// It was lost in the reboot and must be recreated
	dir, err := c.runtime.storageService.GetRunDir(c.ID())
	if err != nil {
		return errors.Wrapf(err, "error retrieving temporary directory for container %s", c.ID())
	}

	if len(c.config.IDMappings.UIDMap) != 0 || len(c.config.IDMappings.GIDMap) != 0 {
		info, err := os.Stat(c.runtime.config.TmpDir)
		if err != nil {
			return errors.Wrapf(err, "cannot stat `%s`", c.runtime.config.TmpDir)
		}
		if err := os.Chmod(c.runtime.config.TmpDir, info.Mode()|0111); err != nil {
			return errors.Wrapf(err, "cannot chmod `%s`", c.runtime.config.TmpDir)
		}
		root := filepath.Join(c.runtime.config.TmpDir, "containers-root", c.ID())
		if err := os.MkdirAll(root, 0755); err != nil {
			return errors.Wrapf(err, "error creating userNS tmpdir for container %s", c.ID())
		}
		if err := os.Chown(root, c.RootUID(), c.RootGID()); err != nil {
			return err
		}
		c.state.UserNSRoot, err = filepath.EvalSymlinks(root)
		if err != nil {
			return errors.Wrapf(err, "failed to eval symlinks for %s", root)
		}
	}

	c.state.RunDir = dir
	c.state.DestinationRunDir = c.state.RunDir
	if c.state.UserNSRoot != "" {
		c.state.DestinationRunDir = filepath.Join(c.state.UserNSRoot, "rundir")
	}

	if err := c.save(); err != nil {
		return errors.Wrapf(err, "error refreshing state for container %s", c.ID())
	}

	// Remove ctl and attach files, which may persist across reboot
	if err := c.removeConmonFiles(); err != nil {
		return err
	}

	return nil
}

// Remove conmon attach socket and terminal resize FIFO
// This is necessary for restarting containers
func (c *Container) removeConmonFiles() error {
	// Files are allowed to not exist, so ignore ENOENT
	attachFile := filepath.Join(c.bundlePath(), "attach")
	if err := os.Remove(attachFile); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "error removing container %s attach file", c.ID())
	}

	ctlFile := filepath.Join(c.bundlePath(), "ctl")
	if err := os.Remove(ctlFile); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "error removing container %s ctl file", c.ID())
	}

	oomFile := filepath.Join(c.bundlePath(), "oom")
	if err := os.Remove(oomFile); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "error removing container %s OOM file", c.ID())
	}

	exitFile := filepath.Join(c.runtime.ociRuntime.exitsDir, c.ID())
	if err := os.Remove(exitFile); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "error removing container %s exit file", c.ID())
	}

	return nil
}

func (c *Container) export(path string) error {
	mountPoint := c.state.Mountpoint
	if !c.state.Mounted {
		mount, err := c.runtime.store.Mount(c.ID(), c.config.MountLabel)
		if err != nil {
			return errors.Wrapf(err, "error mounting container %q", c.ID())
		}
		mountPoint = mount
		defer func() {
			if _, err := c.runtime.store.Unmount(c.ID(), false); err != nil {
				logrus.Errorf("error unmounting container %q: %v", c.ID(), err)
			}
		}()
	}

	input, err := archive.Tar(mountPoint, archive.Uncompressed)
	if err != nil {
		return errors.Wrapf(err, "error reading container directory %q", c.ID())
	}

	outFile, err := os.Create(path)
	if err != nil {
		return errors.Wrapf(err, "error creating file %q", path)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, input)
	return err
}

// Get path of artifact with a given name for this container
func (c *Container) getArtifactPath(name string) string {
	return filepath.Join(c.config.StaticDir, artifactsDir, name)
}

// Used with Wait() to determine if a container has exited
func (c *Container) isStopped() (bool, error) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()
	}
	err := c.syncContainer()
	if err != nil {
		return true, err
	}
	return (c.state.State == ContainerStateStopped || c.state.State == ContainerStateExited), nil
}

// save container state to the database
func (c *Container) save() error {
	if err := c.runtime.state.SaveContainer(c); err != nil {
		return errors.Wrapf(err, "error saving container %s state", c.ID())
	}
	return nil
}

// Check if a container's dependencies are running
// Returns a []string containing the IDs of dependencies that are not running
func (c *Container) checkDependenciesRunning() ([]string, error) {
	deps := c.Dependencies()
	notRunning := []string{}

	// We were not passed a set of dependency containers
	// Make it ourselves
	depCtrs := make(map[string]*Container, len(deps))
	for _, dep := range deps {
		// Get the dependency container
		depCtr, err := c.runtime.state.Container(dep)
		if err != nil {
			return nil, errors.Wrapf(err, "error retrieving dependency %s of container %s from state", dep, c.ID())
		}

		// Check the status
		state, err := depCtr.State()
		if err != nil {
			return nil, errors.Wrapf(err, "error retrieving state of dependency %s of container %s", dep, c.ID())
		}
		if state != ContainerStateRunning {
			notRunning = append(notRunning, dep)
		}
		depCtrs[dep] = depCtr
	}

	return notRunning, nil
}

// Check if a container's dependencies are running
// Returns a []string containing the IDs of dependencies that are not running
// Assumes depencies are already locked, and will be passed in
// Accepts a map[string]*Container containing, at a minimum, the locked
// dependency containers
// (This must be a map from container ID to container)
func (c *Container) checkDependenciesRunningLocked(depCtrs map[string]*Container) ([]string, error) {
	deps := c.Dependencies()
	notRunning := []string{}

	for _, dep := range deps {
		depCtr, ok := depCtrs[dep]
		if !ok {
			return nil, errors.Wrapf(ErrNoSuchCtr, "container %s depends on container %s but it is not on containers passed to checkDependenciesRunning", c.ID(), dep)
		}

		if err := c.syncContainer(); err != nil {
			return nil, err
		}

		if depCtr.state.State != ContainerStateRunning {
			notRunning = append(notRunning, dep)
		}
	}

	return notRunning, nil
}

func (c *Container) completeNetworkSetup() error {
	if !c.config.PostConfigureNetNS {
		return nil
	}
	if err := c.syncContainer(); err != nil {
		return err
	}
	if rootless.IsRootless() {
		return c.runtime.setupRootlessNetNS(c)
	}
	return c.runtime.setupNetNS(c)
}

// Initialize a container, creating it in the runtime
func (c *Container) init(ctx context.Context) error {
	if err := c.makeBindMounts(); err != nil {
		return err
	}

	// Generate the OCI spec
	spec, err := c.generateSpec(ctx)
	if err != nil {
		return err
	}

	// Save the OCI spec to disk
	if err := c.saveSpec(spec); err != nil {
		return err
	}

	// With the spec complete, do an OCI create
	if err := c.runtime.ociRuntime.createContainer(c, c.config.CgroupParent, false); err != nil {
		return err
	}

	logrus.Debugf("Created container %s in OCI runtime", c.ID())

	c.state.ExitCode = 0
	c.state.Exited = false
	c.state.State = ContainerStateCreated

	if err := c.save(); err != nil {
		return err
	}

	return c.completeNetworkSetup()
}

// Clean up a container in the OCI runtime.
// Deletes the container in the runtime, and resets its state to Exited.
// The container can be restarted cleanly after this.
func (c *Container) cleanupRuntime(ctx context.Context) error {
	// If the container is not ContainerStateStopped, do nothing
	if c.state.State != ContainerStateStopped {
		return nil
	}

	// If necessary, delete attach and ctl files
	if err := c.removeConmonFiles(); err != nil {
		return err
	}

	if err := c.delete(ctx); err != nil {
		return err
	}

	// Our state is now Exited, as we've removed ourself from
	// the runtime.
	c.state.State = ContainerStateExited

	if c.valid {
		if err := c.save(); err != nil {
			return err
		}
	}

	logrus.Debugf("Successfully cleaned up container %s", c.ID())

	return nil
}

// Reinitialize a container.
// Deletes and recreates a container in the runtime.
// Should only be done on ContainerStateStopped containers.
// Not necessary for ContainerStateExited - the container has already been
// removed from the runtime, so init() can proceed freely.
func (c *Container) reinit(ctx context.Context) error {
	logrus.Debugf("Recreating container %s in OCI runtime", c.ID())

	if err := c.cleanupRuntime(ctx); err != nil {
		return err
	}

	// Initialize the container again
	return c.init(ctx)
}

// Initialize (if necessary) and start a container
// Performs all necessary steps to start a container that is not running
// Does not lock or check validity
func (c *Container) initAndStart(ctx context.Context) (err error) {
	// If we are ContainerStateUnknown, throw an error
	if c.state.State == ContainerStateUnknown {
		return errors.Wrapf(ErrCtrStateInvalid, "container %s is in an unknown state", c.ID())
	}

	// If we are running, do nothing
	if c.state.State == ContainerStateRunning {
		return nil
	}
	// If we are paused, throw an error
	if c.state.State == ContainerStatePaused {
		return errors.Wrapf(ErrCtrStateInvalid, "cannot start paused container %s", c.ID())
	}

	if err := c.prepare(); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if err2 := c.cleanup(ctx); err2 != nil {
				logrus.Errorf("error cleaning up container %s: %v", c.ID(), err2)
			}
		}
	}()

	// If we are ContainerStateStopped we need to remove from runtime
	// And reset to ContainerStateConfigured
	if c.state.State == ContainerStateStopped {
		logrus.Debugf("Recreating container %s in OCI runtime", c.ID())

		if err := c.reinit(ctx); err != nil {
			return err
		}
	} else if c.state.State == ContainerStateConfigured ||
		c.state.State == ContainerStateExited {
		if err := c.init(ctx); err != nil {
			return err
		}
	}

	// Now start the container
	return c.start()
}

// Internal, non-locking function to start a container
func (c *Container) start() error {
	if err := c.runtime.ociRuntime.startContainer(c); err != nil {
		return err
	}
	logrus.Debugf("Started container %s", c.ID())

	c.state.State = ContainerStateRunning

	return c.save()
}

// Internal, non-locking function to stop container
func (c *Container) stop(timeout uint) error {
	logrus.Debugf("Stopping ctr %s with timeout %d", c.ID(), timeout)

	if err := c.runtime.ociRuntime.stopContainer(c, timeout); err != nil {
		return err
	}

	// Sync the container's state to pick up return code
	if err := c.runtime.ociRuntime.updateContainerStatus(c); err != nil {
		return err
	}

	// Container should clean itself up
	return nil
}

// Internal, non-locking function to pause a container
func (c *Container) pause() error {
	if err := c.runtime.ociRuntime.pauseContainer(c); err != nil {
		return err
	}

	logrus.Debugf("Paused container %s", c.ID())

	c.state.State = ContainerStatePaused

	return c.save()
}

// Internal, non-locking function to unpause a container
func (c *Container) unpause() error {
	if err := c.runtime.ociRuntime.unpauseContainer(c); err != nil {
		return err
	}

	logrus.Debugf("Unpaused container %s", c.ID())

	c.state.State = ContainerStateRunning

	return c.save()
}

// Internal, non-locking function to restart a container
func (c *Container) restartWithTimeout(ctx context.Context, timeout uint) (err error) {
	if c.state.State == ContainerStateUnknown || c.state.State == ContainerStatePaused {
		return errors.Wrapf(ErrCtrStateInvalid, "unable to restart a container in a paused or unknown state")
	}

	if c.state.State == ContainerStateRunning {
		if err := c.stop(timeout); err != nil {
			return err
		}
	}
	if err := c.prepare(); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if err2 := c.cleanup(ctx); err2 != nil {
				logrus.Errorf("error cleaning up container %s: %v", c.ID(), err2)
			}
		}
	}()

	if c.state.State == ContainerStateStopped {
		// Reinitialize the container if we need to
		if err := c.reinit(ctx); err != nil {
			return err
		}
	} else if c.state.State == ContainerStateConfigured ||
		c.state.State == ContainerStateExited {
		// Initialize the container
		if err := c.init(ctx); err != nil {
			return err
		}
	}

	return c.start()
}

// mountStorage sets up the container's root filesystem
// It mounts the image and any other requested mounts
// TODO: Add ability to override mount label so we can use this for Mount() too
// TODO: Can we use this for export? Copying SHM into the export might not be
// good
func (c *Container) mountStorage() (err error) {
	// Container already mounted, nothing to do
	if c.state.Mounted {
		return nil
	}

	if !rootless.IsRootless() {
		// TODO: generalize this mount code so it will mount every mount in ctr.config.Mounts
		mounted, err := mount.Mounted(c.config.ShmDir)
		if err != nil {
			return errors.Wrapf(err, "unable to determine if %q is mounted", c.config.ShmDir)
		}

		if err := os.Chown(c.config.ShmDir, c.RootUID(), c.RootGID()); err != nil {
			return errors.Wrapf(err, "failed to chown %s", c.config.ShmDir)
		}

		if !mounted {
			shmOptions := fmt.Sprintf("mode=1777,size=%d", c.config.ShmSize)
			if err := c.mountSHM(shmOptions); err != nil {
				return err
			}
			if err := os.Chown(c.config.ShmDir, c.RootUID(), c.RootGID()); err != nil {
				return errors.Wrapf(err, "failed to chown %s", c.config.ShmDir)
			}
		}
	}

	mountPoint := c.config.Rootfs
	if mountPoint == "" {
		mountPoint, err = c.mount()
		if err != nil {
			return err
		}
	}
	c.state.Mounted = true
	c.state.Mountpoint = mountPoint
	if c.state.UserNSRoot == "" {
		c.state.RealMountpoint = c.state.Mountpoint
	} else {
		c.state.RealMountpoint = filepath.Join(c.state.UserNSRoot, "mountpoint")
	}

	logrus.Debugf("Created root filesystem for container %s at %s", c.ID(), c.state.Mountpoint)

	defer func() {
		if err != nil {
			if err2 := c.cleanupStorage(); err2 != nil {
				logrus.Errorf("Error unmounting storage for container %s: %v", c.ID(), err)
			}
		}
	}()

	return c.save()
}

// cleanupStorage unmounts and cleans up the container's root filesystem
func (c *Container) cleanupStorage() error {
	if !c.state.Mounted {
		// Already unmounted, do nothing
		logrus.Debugf("Storage is already unmounted, skipping...")
		return nil
	}
	for _, mount := range c.config.Mounts {
		if err := c.unmountSHM(mount); err != nil {
			return err
		}
	}
	if c.config.Rootfs != "" {
		return nil
	}

	if err := c.unmount(false); err != nil {
		// If the container has already been removed, warn but don't
		// error
		// We still want to be able to kick the container out of the
		// state
		if err == storage.ErrNotAContainer || err == storage.ErrContainerUnknown {
			logrus.Errorf("Storage for container %s has been removed", c.ID())
			return nil
		}

		return err
	}

	c.state.Mountpoint = ""
	c.state.Mounted = false

	if c.valid {
		return c.save()
	}
	return nil
}

// Unmount the a container and free its resources
func (c *Container) cleanup(ctx context.Context) error {
	var lastError error

	logrus.Debugf("Cleaning up container %s", c.ID())

	// Clean up network namespace, if present
	if err := c.cleanupNetwork(); err != nil {
		lastError = err
	}

	// Unmount storage
	if err := c.cleanupStorage(); err != nil {
		if lastError != nil {
			logrus.Errorf("Error unmounting container %s storage: %v", c.ID(), err)
		} else {
			lastError = err
		}
	}

	// Remove the container from the runtime, if necessary
	if err := c.cleanupRuntime(ctx); err != nil {
		if lastError != nil {
			logrus.Errorf("Error removing container %s from OCI runtime: %v", c.ID(), err)
		} else {
			lastError = err
		}
	}

	return lastError
}

// delete deletes the container and runs any configured poststop
// hooks.
func (c *Container) delete(ctx context.Context) (err error) {
	if err := c.runtime.ociRuntime.deleteContainer(c); err != nil {
		return errors.Wrapf(err, "error removing container %s from runtime", c.ID())
	}

	if err := c.postDeleteHooks(ctx); err != nil {
		return errors.Wrapf(err, "container %s poststop hooks", c.ID())
	}

	return nil
}

// postDeleteHooks runs the poststop hooks (if any) as specified by
// the OCI Runtime Specification (which requires them to run
// post-delete, despite the stage name).
func (c *Container) postDeleteHooks(ctx context.Context) (err error) {
	if c.state.ExtensionStageHooks != nil {
		extensionHooks, ok := c.state.ExtensionStageHooks["poststop"]
		if ok {
			state, err := json.Marshal(spec.State{
				Version:     spec.Version,
				ID:          c.ID(),
				Status:      "stopped",
				Bundle:      c.bundlePath(),
				Annotations: c.config.Spec.Annotations,
			})
			if err != nil {
				return err
			}
			for i, hook := range extensionHooks {
				logrus.Debugf("container %s: invoke poststop hook %d, path %s", c.ID(), i, hook.Path)
				var stderr, stdout bytes.Buffer
				hookErr, err := exec.Run(ctx, &hook, state, &stdout, &stderr, exec.DefaultPostKillTimeout)
				if err != nil {
					logrus.Warnf("container %s: poststop hook %d: %v", c.ID(), i, err)
					if hookErr != err {
						logrus.Debugf("container %s: poststop hook %d (hook error): %v", c.ID(), i, hookErr)
					}
					stdoutString := stdout.String()
					if stdoutString != "" {
						logrus.Debugf("container %s: poststop hook %d: stdout:\n%s", c.ID(), i, stdoutString)
					}
					stderrString := stderr.String()
					if stderrString != "" {
						logrus.Debugf("container %s: poststop hook %d: stderr:\n%s", c.ID(), i, stderrString)
					}
				}
			}
		}
	}

	return nil
}

// Make standard bind mounts to include in the container
func (c *Container) makeBindMounts() error {
	if err := os.Chown(c.state.RunDir, c.RootUID(), c.RootGID()); err != nil {
		return errors.Wrapf(err, "cannot chown run directory %s", c.state.RunDir)
	}

	if c.state.BindMounts == nil {
		c.state.BindMounts = make(map[string]string)
	}

	// SHM is always added when we mount the container
	c.state.BindMounts["/dev/shm"] = c.config.ShmDir

	// Make /etc/resolv.conf
	if _, ok := c.state.BindMounts["/etc/resolv.conf"]; ok {
		// If it already exists, delete so we can recreate
		delete(c.state.BindMounts, "/etc/resolv.conf")
	}
	newResolv, err := c.generateResolvConf()
	if err != nil {
		return errors.Wrapf(err, "error creating resolv.conf for container %s", c.ID())
	}
	c.state.BindMounts["/etc/resolv.conf"] = newResolv

	// Make /etc/hosts
	if _, ok := c.state.BindMounts["/etc/hosts"]; ok {
		// If it already exists, delete so we can recreate
		delete(c.state.BindMounts, "/etc/hosts")
	}
	newHosts, err := c.generateHosts()
	if err != nil {
		return errors.Wrapf(err, "error creating hosts file for container %s", c.ID())
	}
	c.state.BindMounts["/etc/hosts"] = newHosts

	// Make /etc/hostname
	// This should never change, so no need to recreate if it exists
	if _, ok := c.state.BindMounts["/etc/hostname"]; !ok {
		hostnamePath, err := c.writeStringToRundir("hostname", c.Hostname())
		if err != nil {
			return errors.Wrapf(err, "error creating hostname file for container %s", c.ID())
		}
		c.state.BindMounts["/etc/hostname"] = hostnamePath
	}

	// Make .containerenv
	// Empty file, so no need to recreate if it exists
	if _, ok := c.state.BindMounts["/run/.containerenv"]; !ok {
		// Empty string for now, but we may consider populating this later
		containerenvPath, err := c.writeStringToRundir(".containerenv", "")
		if err != nil {
			return errors.Wrapf(err, "error creating containerenv file for container %s", c.ID())
		}
		c.state.BindMounts["/run/.containerenv"] = containerenvPath
	}

	// Add Secret Mounts
	secretMounts := secrets.SecretMountsWithUIDGID(c.config.MountLabel, c.state.RunDir, c.runtime.config.DefaultMountsFile, c.state.DestinationRunDir, c.RootUID(), c.RootGID())
	for _, mount := range secretMounts {
		if _, ok := c.state.BindMounts[mount.Destination]; !ok {
			c.state.BindMounts[mount.Destination] = mount.Source
		}
	}

	return nil
}

// writeStringToRundir copies the provided file to the runtimedir
func (c *Container) writeStringToRundir(destFile, output string) (string, error) {
	destFileName := filepath.Join(c.state.RunDir, destFile)

	if err := os.Remove(destFileName); err != nil && !os.IsNotExist(err) {
		return "", errors.Wrapf(err, "error removing %s for container %s", destFile, c.ID())
	}

	f, err := os.Create(destFileName)
	if err != nil {
		return "", errors.Wrapf(err, "unable to create %s", destFileName)
	}
	defer f.Close()
	if err := f.Chown(c.RootUID(), c.RootGID()); err != nil {
		return "", err
	}

	if _, err := f.WriteString(output); err != nil {
		return "", errors.Wrapf(err, "unable to write %s", destFileName)
	}
	// Relabel runDirResolv for the container
	if err := label.Relabel(destFileName, c.config.MountLabel, false); err != nil {
		return "", err
	}

	return filepath.Join(c.state.DestinationRunDir, destFile), nil
}

// generateResolvConf generates a containers resolv.conf
func (c *Container) generateResolvConf() (string, error) {
	// Determine the endpoint for resolv.conf in case it is a symlink
	resolvPath, err := filepath.EvalSymlinks("/etc/resolv.conf")
	if err != nil {
		return "", err
	}

	contents, err := ioutil.ReadFile(resolvPath)
	if err != nil {
		return "", errors.Wrapf(err, "unable to read %s", resolvPath)
	}

	// Process the file to remove localhost nameservers
	// TODO: set ipv6 enable bool more sanely
	resolv, err := resolvconf.FilterResolvDNS(contents, true)
	if err != nil {
		return "", errors.Wrapf(err, "error parsing host resolv.conf")
	}

	// Make a new resolv.conf
	nameservers := resolvconf.GetNameservers(resolv.Content)
	if len(c.config.DNSServer) > 0 {
		// We store DNS servers as net.IP, so need to convert to string
		nameservers = []string{}
		for _, server := range c.config.DNSServer {
			nameservers = append(nameservers, server.String())
		}
	}

	search := resolvconf.GetSearchDomains(resolv.Content)
	if len(c.config.DNSSearch) > 0 {
		search = c.config.DNSSearch
	}

	options := resolvconf.GetOptions(resolv.Content)
	if len(c.config.DNSOption) > 0 {
		options = c.config.DNSOption
	}

	destPath := filepath.Join(c.state.RunDir, "resolv.conf")

	if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
		return "", errors.Wrapf(err, "error removing resolv.conf for container %s", c.ID())
	}

	// Build resolv.conf
	if _, err = resolvconf.Build(destPath, nameservers, search, options); err != nil {
		return "", errors.Wrapf(err, "error building resolv.conf for container %s")
	}

	return destPath, nil
}

// generateHosts creates a containers hosts file
func (c *Container) generateHosts() (string, error) {
	orig, err := ioutil.ReadFile("/etc/hosts")
	if err != nil {
		return "", errors.Wrapf(err, "unable to read /etc/hosts")
	}
	hosts := string(orig)
	if len(c.config.HostAdd) > 0 {
		for _, host := range c.config.HostAdd {
			// the host format has already been verified at this point
			fields := strings.SplitN(host, ":", 2)
			hosts += fmt.Sprintf("%s %s\n", fields[1], fields[0])
		}
	}
	return c.writeStringToRundir("hosts", hosts)
}

func (c *Container) addLocalVolumes(ctx context.Context, g *generate.Generator) error {
	mountPoint := c.state.Mountpoint
	if !c.state.Mounted {
		return errors.Wrapf(ErrInternal, "container is not mounted")
	}
	newImage, err := c.runtime.imageRuntime.NewFromLocal(c.config.RootfsImageID)
	if err != nil {
		return err
	}
	imageData, err := newImage.Inspect(ctx)
	if err != nil {
		return err
	}
	// Add the built-in volumes of the container passed in to --volumes-from
	for _, vol := range c.config.LocalVolumes {
		if imageData.ContainerConfig.Volumes == nil {
			imageData.ContainerConfig.Volumes = map[string]struct{}{
				vol: {},
			}
		} else {
			imageData.ContainerConfig.Volumes[vol] = struct{}{}
		}
	}

	for k := range imageData.ContainerConfig.Volumes {
		mount := spec.Mount{
			Destination: k,
			Type:        "bind",
			Options:     []string{"private", "bind", "rw"},
		}
		if MountExists(g.Mounts(), k) {
			continue
		}
		volumePath := filepath.Join(c.config.StaticDir, "volumes", k)
		srcPath := filepath.Join(mountPoint, k)

		var (
			uid uint32
			gid uint32
		)
		if c.config.User != "" {
			if !c.state.Mounted {
				return errors.Wrapf(ErrCtrStateInvalid, "container %s must be mounted in order to translate User field", c.ID())
			}
			uid, gid, err = chrootuser.GetUser(c.state.Mountpoint, c.config.User)
			if err != nil {
				return err
			}
		}

		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			logrus.Infof("Volume image mount point %s does not exist in root FS, need to create it", k)
			if err = os.MkdirAll(srcPath, 0755); err != nil {
				return errors.Wrapf(err, "error creating directory %q for volume %q in container %q", volumePath, k, c.ID)
			}

			if err = os.Chown(srcPath, int(uid), int(gid)); err != nil {
				return errors.Wrapf(err, "error chowning directory %q for volume %q in container %q", srcPath, k, c.ID)
			}
		}

		if _, err := os.Stat(volumePath); os.IsNotExist(err) {
			if err = os.MkdirAll(volumePath, 0755); err != nil {
				return errors.Wrapf(err, "error creating directory %q for volume %q in container %q", volumePath, k, c.ID)
			}

			if err = os.Chown(volumePath, int(uid), int(gid)); err != nil {
				return errors.Wrapf(err, "error chowning directory %q for volume %q in container %q", volumePath, k, c.ID)
			}

			if err = label.Relabel(volumePath, c.config.MountLabel, false); err != nil {
				return errors.Wrapf(err, "error relabeling directory %q for volume %q in container %q", volumePath, k, c.ID)
			}
			if err = chrootarchive.NewArchiver(nil).CopyWithTar(srcPath, volumePath); err != nil && !os.IsNotExist(err) {
				return errors.Wrapf(err, "error populating directory %q for volume %q in container %q using contents of %q", volumePath, k, c.ID, srcPath)
			}

			// Set the volume path with the same owner and permission of source path
			sstat, _ := os.Stat(srcPath)
			st, ok := sstat.Sys().(*syscall.Stat_t)
			if !ok {
				return fmt.Errorf("could not convert to syscall.Stat_t")
			}
			uid := int(st.Uid)
			gid := int(st.Gid)

			if err := os.Lchown(volumePath, uid, gid); err != nil {
				return err
			}
			if os.Chmod(volumePath, sstat.Mode()); err != nil {
				return err
			}

		}

		mount.Source = volumePath
		g.AddMount(mount)
	}
	return nil
}

// Save OCI spec to disk, replacing any existing specs for the container
func (c *Container) saveSpec(spec *spec.Spec) error {
	// If the OCI spec already exists, we need to replace it
	// Cannot guarantee some things, e.g. network namespaces, have the same
	// paths
	jsonPath := filepath.Join(c.bundlePath(), "config.json")
	if _, err := os.Stat(jsonPath); err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrapf(err, "error doing stat on container %s spec", c.ID())
		}
		// The spec does not exist, we're fine
	} else {
		// The spec exists, need to remove it
		if err := os.Remove(jsonPath); err != nil {
			return errors.Wrapf(err, "error replacing runtime spec for container %s", c.ID())
		}
	}

	fileJSON, err := json.Marshal(spec)
	if err != nil {
		return errors.Wrapf(err, "error exporting runtime spec for container %s to JSON", c.ID())
	}
	if err := ioutil.WriteFile(jsonPath, fileJSON, 0644); err != nil {
		return errors.Wrapf(err, "error writing runtime spec JSON for container %s to disk", c.ID())
	}

	logrus.Debugf("Created OCI spec for container %s at %s", c.ID(), jsonPath)

	c.state.ConfigPath = jsonPath

	return nil
}

func (c *Container) setupOCIHooks(ctx context.Context, config *spec.Spec) (extensionStageHooks map[string][]spec.Hook, err error) {
	if len(c.runtime.config.HooksDir) == 0 {
		return nil, nil
	}

	var locale string
	var ok bool
	for _, envVar := range []string{
		"LC_ALL",
		"LC_COLLATE",
		"LANG",
	} {
		locale, ok = os.LookupEnv(envVar)
		if ok {
			break
		}
	}

	langString, ok := localeToLanguage[strings.ToLower(locale)]
	if !ok {
		langString = locale
	}

	lang, err := language.Parse(langString)
	if err != nil {
		logrus.Warnf("failed to parse language %q: %s", langString, err)
		lang, err = language.Parse("und-u-va-posix")
		if err != nil {
			return nil, err
		}
	}

	allHooks := make(map[string][]spec.Hook)
	for _, hDir := range c.runtime.config.HooksDir {
		manager, err := hooks.New(ctx, []string{hDir}, []string{"poststop"}, lang)
		if err != nil {
			if c.runtime.config.HooksDirNotExistFatal || !os.IsNotExist(err) {
				return nil, err
			}
			logrus.Warnf("failed to load hooks: {}", err)
			return nil, nil
		}
		hooks, err := manager.Hooks(config, c.Spec().Annotations, len(c.config.UserVolumes) > 0)
		if err != nil {
			return nil, err
		}
		for i, hook := range hooks {
			allHooks[i] = hook
		}
	}
	return allHooks, nil
}

// mount mounts the container's root filesystem
func (c *Container) mount() (string, error) {
	mountPoint, err := c.runtime.storageService.MountContainerImage(c.ID())
	if err != nil {
		return "", errors.Wrapf(err, "error mounting storage for container %s", c.ID())
	}
	mountPoint, err = filepath.EvalSymlinks(mountPoint)
	if err != nil {
		return "", errors.Wrapf(err, "error resolving storage path for container %s", c.ID())
	}
	return mountPoint, nil
}

// unmount unmounts the container's root filesystem
func (c *Container) unmount(force bool) error {
	// Also unmount storage
	if _, err := c.runtime.storageService.UnmountContainerImage(c.ID(), force); err != nil {
		return errors.Wrapf(err, "error unmounting container %s root filesystem", c.ID())
	}

	return nil
}

// getExcludedCGroups returns a string slice of cgroups we want to exclude
// because runc or other components are unaware of them.
func getExcludedCGroups() (excludes []string) {
	excludes = []string{"rdma"}
	return
}
