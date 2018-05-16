package libpod

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/cgroups"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/chrootarchive"
	"github.com/containers/storage/pkg/idtools"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/stringid"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	crioAnnotations "github.com/projectatomic/libpod/pkg/annotations"
	"github.com/projectatomic/libpod/pkg/chrootuser"
	"github.com/projectatomic/libpod/pkg/hooks"
	"github.com/projectatomic/libpod/pkg/secrets"
	"github.com/projectatomic/libpod/pkg/util"
	"github.com/sirupsen/logrus"
	"github.com/ulule/deepcopier"
	"golang.org/x/sys/unix"
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
	// because the parent of the last layer is "", andlstore.Get("")
	// will return an error.
	layerSize, err := c.runtime.store.DiffSize(layer.Parent, layer.ID)
	return size + layerSize, err
}

// rwSize Gets the size of the mutable top layer of the container.
func (c *Container) rwSize() (int64, error) {
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
		(c.state.State != ContainerStateConfigured) {
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

// Make a new container
func newContainer(rspec *spec.Spec, lockDir string) (*Container, error) {
	if rspec == nil {
		return nil, errors.Wrapf(ErrInvalidArg, "must provide a valid runtime spec to create container")
	}

	ctr := new(Container)
	ctr.config = new(ContainerConfig)
	ctr.state = new(containerState)

	ctr.config.ID = stringid.GenerateNonCryptoID()

	ctr.config.Spec = new(spec.Spec)
	deepcopier.Copy(rspec).To(ctr.config.Spec)
	ctr.config.CreatedTime = time.Now()

	ctr.config.ShmSize = DefaultShmSize

	ctr.state.BindMounts = make(map[string]string)

	// Path our lock file will reside at
	lockPath := filepath.Join(lockDir, ctr.config.ID)
	// Grab a lockfile at the given path
	lock, err := storage.GetLockfile(lockPath)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating lockfile for new container")
	}
	ctr.lock = lock

	return ctr, nil
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
	if c.config.RootfsImageID == "" || c.config.RootfsImageName == "" {
		return errors.Wrapf(ErrInvalidArg, "must provide image ID and image name to use an image")
	}

	options := storage.ContainerOptions{IDMappingOptions: c.config.IDMappings}
	containerInfo, err := c.runtime.storageService.CreateContainerStorage(ctx, c.runtime.imageContext, c.config.RootfsImageName, c.config.RootfsImageID, c.config.Name, c.config.ID, c.config.MountLabel, &options)
	if err != nil {
		return errors.Wrapf(err, "error creating container storage")
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

	c.config.StaticDir = containerInfo.Dir
	c.state.RunDir = containerInfo.RunDir
	c.state.DestinationRunDir = c.state.RunDir
	if c.state.UserNSRoot != "" {
		c.state.DestinationRunDir = filepath.Join(c.state.UserNSRoot, "rundir")
	}

	// Set the default Entrypoint and Command
	c.config.Entrypoint = containerInfo.Config.Config.Entrypoint
	c.config.Command = containerInfo.Config.Config.Cmd

	artifacts := filepath.Join(c.config.StaticDir, artifactsDir)
	if err := os.MkdirAll(artifacts, 0755); err != nil {
		return errors.Wrapf(err, "error creating artifacts directory %q", artifacts)
	}

	return nil
}

// Tear down a container's storage prior to removal
func (c *Container) teardownStorage() error {
	if !c.valid {
		return errors.Wrapf(ErrCtrRemoved, "container %s is not valid", c.ID())
	}

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

// Refresh refreshes the container's state after a restart
func (c *Container) refresh() error {
	c.lock.Lock()
	defer c.lock.Unlock()

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
			if err := c.runtime.store.Unmount(c.ID()); err != nil {
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
	return c.state.State == ContainerStateStopped, nil
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
	if err := c.runtime.ociRuntime.createContainer(c, c.config.CgroupParent); err != nil {
		return err
	}

	logrus.Debugf("Created container %s in OCI runtime", c.ID())

	c.state.State = ContainerStateCreated

	if err := c.save(); err != nil {
		return err
	}

	return c.completeNetworkSetup()
}

// Reinitialize a container
// Deletes and recreates a container in the runtime
// Should only be done on ContainerStateStopped containers
func (c *Container) reinit(ctx context.Context) error {
	logrus.Debugf("Recreating container %s in OCI runtime", c.ID())

	// If necessary, delete attach and ctl files
	if err := c.removeConmonFiles(); err != nil {
		return err
	}

	// Delete the container in the runtime
	if err := c.runtime.ociRuntime.deleteContainer(c); err != nil {
		return errors.Wrapf(err, "error removing container %s from runtime", c.ID())
	}
	// Our state is now Configured, as we've removed ourself from
	// the runtime
	// Set and save now to make sure that, if the init() below fails
	// we still have a valid state
	c.state.State = ContainerStateConfigured
	if err := c.save(); err != nil {
		return err
	}

	logrus.Debugf("Successfully cleaned up container %s", c.ID())

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
			if err2 := c.cleanup(); err2 != nil {
				logrus.Errorf("error cleaning up container %s: %v", c.ID(), err2)
			}
		}
	}()

	// If we are ContainerStateStopped we need to remove from runtime
	// And reset to ContainerStateConfigured
	if c.state.State == ContainerStateStopped {
		logrus.Debugf("Recreating container %s in OCI runtime", c.ID())

		// If necessary, delete attach and ctl files
		if err := c.removeConmonFiles(); err != nil {
			return err
		}

		// Delete the container in the runtime
		if err := c.runtime.ociRuntime.deleteContainer(c); err != nil {
			return errors.Wrapf(err, "error removing container %s from runtime", c.ID())
		}

		// Our state is now Configured, as we've removed ourself from
		// the runtime
		// Set and save now to make sure that, if the init() below fails
		// we still have a valid state
		c.state.State = ContainerStateConfigured
		if err := c.save(); err != nil {
			return err
		}
	}

	// If we are ContainerStateConfigured we need to init()
	if c.state.State == ContainerStateConfigured {
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

	return c.cleanupStorage()
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

	// TODO: generalize this mount code so it will mount every mount in ctr.config.Mounts

	mounted, err := mount.Mounted(c.config.ShmDir)
	if err != nil {
		return errors.Wrapf(err, "unable to determine if %q is mounted", c.config.ShmDir)
	}

	if err := os.Chown(c.config.ShmDir, c.RootUID(), c.RootGID()); err != nil {
		return err
	}

	if !mounted {
		shmOptions := fmt.Sprintf("mode=1777,size=%d", c.config.ShmSize)
		if err := unix.Mount("shm", c.config.ShmDir, "tmpfs", unix.MS_NOEXEC|unix.MS_NOSUID|unix.MS_NODEV,
			label.FormatMountLabel(shmOptions, c.config.MountLabel)); err != nil {
			return errors.Wrapf(err, "failed to mount shm tmpfs %q", c.config.ShmDir)
		}
		if err := os.Chown(c.config.ShmDir, c.RootUID(), c.RootGID()); err != nil {
			return errors.Wrapf(err, "failed to chown %s", c.config.ShmDir)
		}
	}

	mountPoint, err := c.runtime.storageService.MountContainerImage(c.ID())
	if err != nil {
		return errors.Wrapf(err, "error mounting storage for container %s", c.ID())
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

// prepare mounts the container and sets up other required resources like net
// namespaces
func (c *Container) prepare() (err error) {
	// Mount storage if not mounted
	if err := c.mountStorage(); err != nil {
		return err
	}

	// Set up network namespace if not already set up
	if c.config.CreateNetNS && c.state.NetNS == nil && !c.config.PostConfigureNetNS {
		if err := c.runtime.createNetNS(c); err != nil {
			// Tear down storage before exiting to make sure we
			// don't leak mounts
			if err2 := c.cleanupStorage(); err2 != nil {
				logrus.Errorf("Error cleaning up storage for container %s: %v", c.ID(), err2)
			}
			return err
		}
	}

	return nil
}

// cleanupCgroup cleans up residual CGroups after container execution
// This is a no-op for the systemd cgroup driver
func (c *Container) cleanupCgroups() error {
	if c.runtime.config.CgroupManager == SystemdCgroupsManager {
		return nil
	}

	// Remove the base path of the container's cgroups
	path := filepath.Join(c.config.CgroupParent, fmt.Sprintf("libpod-%s", c.ID()))

	logrus.Debugf("Removing CGroup %s", path)

	cgroup, err := cgroups.Load(cgroups.V1, cgroups.StaticPath(path))
	if err != nil {
		// It's fine for the cgroup to not exist
		// We want it gone, it's gone
		if err == cgroups.ErrCgroupDeleted {
			return nil
		}

		return err
	}

	if err := cgroup.Delete(); err != nil {
		return err
	}

	return nil
}

// cleanupNetwork unmounts and cleans up the container's network
func (c *Container) cleanupNetwork() error {
	// Stop the container's network namespace (if it has one)
	if err := c.runtime.teardownNetNS(c); err != nil {
		logrus.Errorf("unable to cleanup network for container %s: %q", c.ID(), err)
	}

	c.state.NetNS = nil
	c.state.IPs = nil
	c.state.Routes = nil
	return c.save()
}

// cleanupStorage unmounts and cleans up the container's root filesystem
func (c *Container) cleanupStorage() error {
	if !c.state.Mounted {
		// Already unmounted, do nothing
		return nil
	}

	for _, mount := range c.config.Mounts {
		if err := unix.Unmount(mount, unix.MNT_DETACH); err != nil {
			if err != syscall.EINVAL {
				logrus.Warnf("container %s failed to unmount %s : %v", c.ID(), mount, err)
			}
		}
	}

	// Also unmount storage
	if err := c.runtime.storageService.UnmountContainerImage(c.ID()); err != nil {
		// If the container has already been removed, warn but don't
		// error
		// We still want to be able to kick the container out of the
		// state
		if err == storage.ErrNotAContainer || err == storage.ErrContainerUnknown {
			logrus.Errorf("Storage for container %s has been removed", c.ID())
			return nil
		}

		return errors.Wrapf(err, "error unmounting container %s root filesystem", c.ID())
	}

	c.state.Mountpoint = ""
	c.state.Mounted = false

	return c.save()
}

// Unmount the a container and free its resources
func (c *Container) cleanup() error {
	var lastError error

	logrus.Debugf("Cleaning up container %s", c.ID())

	// Clean up network namespace, if present
	if err := c.cleanupNetwork(); err != nil {
		lastError = nil
	}

	if err := c.cleanupCgroups(); err != nil {
		if lastError != nil {
			logrus.Errorf("Error cleaning up container %s CGroups: %v", c.ID(), err)
		} else {
			lastError = err
		}
	}

	// Unmount storage
	if err := c.cleanupStorage(); err != nil {
		if lastError != nil {
			logrus.Errorf("Error unmounting container %s storage: %v", c.ID(), err)
		} else {
			lastError = err
		}
	}

	return lastError
}

// Make standard bind mounts to include in the container
func (c *Container) makeBindMounts() error {
	if err := os.Chown(c.state.RunDir, c.RootUID(), c.RootGID()); err != nil {
		return errors.Wrapf(err, "error chown %s", c.state.RunDir)
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

type resolvConf struct {
	nameServers   []string
	searchDomains []string
	options       []string
}

// generateResolvConf generates a containers resolv.conf
func (c *Container) generateResolvConf() (string, error) {
	// Copy /etc/resolv.conf to the container's rundir
	resolvPath := "/etc/resolv.conf"

	// Check if the host system is using system resolve and if so
	// copy its resolv.conf
	if _, err := os.Stat("/run/systemd/resolve/resolv.conf"); err == nil {
		resolvPath = "/run/systemd/resolve/resolv.conf"
	}
	orig, err := ioutil.ReadFile(resolvPath)
	if err != nil {
		return "", errors.Wrapf(err, "unable to read %s", resolvPath)
	}
	if len(c.config.DNSServer) == 0 && len(c.config.DNSSearch) == 0 && len(c.config.DNSOption) == 0 {
		return c.writeStringToRundir("resolv.conf", fmt.Sprintf("%s", orig))
	}

	// Read and organize the hosts /etc/resolv.conf
	resolv := createResolv(string(orig[:]))

	// Populate the resolv struct with user's dns search domains
	if len(c.config.DNSSearch) > 0 {
		resolv.searchDomains = nil
		// The . character means the user doesnt want any search domains in the container
		if !util.StringInSlice(".", c.config.DNSSearch) {
			resolv.searchDomains = append(resolv.searchDomains, c.Config().DNSSearch...)
		}
	}

	// Populate the resolv struct with user's dns servers
	if len(c.config.DNSServer) > 0 {
		resolv.nameServers = nil
		for _, i := range c.config.DNSServer {
			resolv.nameServers = append(resolv.nameServers, i.String())
		}
	}

	// Populate the resolve struct with the users dns options
	if len(c.config.DNSOption) > 0 {
		resolv.options = nil
		resolv.options = append(resolv.options, c.Config().DNSOption...)
	}
	return c.writeStringToRundir("resolv.conf", resolv.ToString())
}

// createResolv creates a resolv struct from an input string
func createResolv(input string) resolvConf {
	var resolv resolvConf
	for _, line := range strings.Split(input, "\n") {
		if strings.HasPrefix(line, "search") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				logrus.Debugf("invalid resolv.conf line %s", line)
				continue
			}
			resolv.searchDomains = append(resolv.searchDomains, fields[1:]...)
		} else if strings.HasPrefix(line, "nameserver") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				logrus.Debugf("invalid resolv.conf line %s", line)
				continue
			}
			resolv.nameServers = append(resolv.nameServers, fields[1])
		} else if strings.HasPrefix(line, "options") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				logrus.Debugf("invalid resolv.conf line %s", line)
				continue
			}
			resolv.options = append(resolv.options, fields[1:]...)
		}
	}
	return resolv
}

//ToString returns a resolv struct in the form of a resolv.conf
func (r resolvConf) ToString() string {
	var result string
	// Populate the output string with search domains
	result += fmt.Sprintf("search %s\n", strings.Join(r.searchDomains, " "))
	// Populate the output string with name servers
	for _, i := range r.nameServers {
		result += fmt.Sprintf("nameserver %s\n", i)
	}
	// Populate the output string with dns options
	for _, i := range r.options {
		result += fmt.Sprintf("options %s\n", i)
	}
	return result
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

// Generate spec for a container
// Accepts a map of the container's dependencies
func (c *Container) generateSpec(ctx context.Context) (*spec.Spec, error) {
	g := generate.NewFromSpec(c.config.Spec)

	// If network namespace was requested, add it now
	if c.config.CreateNetNS {
		if c.config.PostConfigureNetNS {
			g.AddOrReplaceLinuxNamespace(spec.NetworkNamespace, "")
		} else {
			g.AddOrReplaceLinuxNamespace(spec.NetworkNamespace, c.state.NetNS.Path())
		}
	}

	// Remove the default /dev/shm mount to ensure we overwrite it
	g.RemoveMount("/dev/shm")

	// Add bind mounts to container
	for dstPath, srcPath := range c.state.BindMounts {
		newMount := spec.Mount{
			Type:        "bind",
			Source:      srcPath,
			Destination: dstPath,
			Options:     []string{"rw", "bind"},
		}
		if !MountExists(g.Mounts(), dstPath) {
			g.AddMount(newMount)
		} else {
			logrus.Warnf("User mount overriding libpod mount at %q", dstPath)
		}
	}

	if err := c.setupOCIHooks(ctx, &g); err != nil {
		return nil, errors.Wrapf(err, "error setting up OCI Hooks")
	}
	// Bind builtin image volumes
	if c.config.ImageVolumes {
		if err := c.addImageVolumes(ctx, &g); err != nil {
			return nil, errors.Wrapf(err, "error mounting image volumes")
		}
	}

	if c.config.User != "" {
		if !c.state.Mounted {
			return nil, errors.Wrapf(ErrCtrStateInvalid, "container %s must be mounted in order to translate User field", c.ID())
		}
		uid, gid, err := chrootuser.GetUser(c.state.Mountpoint, c.config.User)
		if err != nil {
			return nil, err
		}
		// User and Group must go together
		g.SetProcessUID(uid)
		g.SetProcessGID(gid)
	}

	// Add addition groups if c.config.GroupAdd is not empty
	if len(c.config.Groups) > 0 {
		if !c.state.Mounted {
			return nil, errors.Wrapf(ErrCtrStateInvalid, "container %s must be mounted in order to add additional groups", c.ID())
		}
		for _, group := range c.config.Groups {
			gid, err := chrootuser.GetGroup(c.state.Mountpoint, group)
			if err != nil {
				return nil, err
			}
			g.AddProcessAdditionalGid(gid)
		}
	}

	// Look up and add groups the user belongs to, if a group wasn't directly specified
	if !strings.Contains(c.config.User, ":") {
		groups, err := chrootuser.GetAdditionalGroupsForUser(c.state.Mountpoint, uint64(g.Spec().Process.User.UID))
		if err != nil && errors.Cause(err) != chrootuser.ErrNoSuchUser {
			return nil, err
		}
		for _, gid := range groups {
			g.AddProcessAdditionalGid(gid)
		}
	}

	// Add shared namespaces from other containers
	if c.config.IPCNsCtr != "" {
		if err := c.addNamespaceContainer(&g, IPCNS, c.config.IPCNsCtr, spec.IPCNamespace); err != nil {
			return nil, err
		}
	}
	if c.config.MountNsCtr != "" {
		if err := c.addNamespaceContainer(&g, MountNS, c.config.MountNsCtr, spec.MountNamespace); err != nil {
			return nil, err
		}
	}
	if c.config.NetNsCtr != "" {
		if err := c.addNamespaceContainer(&g, NetNS, c.config.NetNsCtr, spec.NetworkNamespace); err != nil {
			return nil, err
		}
	}
	if c.config.PIDNsCtr != "" {
		if err := c.addNamespaceContainer(&g, PIDNS, c.config.PIDNsCtr, string(spec.PIDNamespace)); err != nil {
			return nil, err
		}
	}
	if c.config.UserNsCtr != "" {
		if err := c.addNamespaceContainer(&g, UserNS, c.config.UserNsCtr, spec.UserNamespace); err != nil {
			return nil, err
		}
	}
	if c.config.UTSNsCtr != "" {
		if err := c.addNamespaceContainer(&g, UTSNS, c.config.UTSNsCtr, spec.UTSNamespace); err != nil {
			return nil, err
		}
	}
	if c.config.CgroupNsCtr != "" {
		if err := c.addNamespaceContainer(&g, CgroupNS, c.config.CgroupNsCtr, spec.CgroupNamespace); err != nil {
			return nil, err
		}
	}

	if err := idtools.MkdirAllAs(c.state.RealMountpoint, 0700, c.RootUID(), c.RootGID()); err != nil {
		return nil, err
	}

	g.SetRootPath(c.state.RealMountpoint)
	g.AddAnnotation(crioAnnotations.Created, c.config.CreatedTime.Format(time.RFC3339Nano))
	g.AddAnnotation("org.opencontainers.image.stopSignal", fmt.Sprintf("%d", c.config.StopSignal))

	g.SetHostname(c.Hostname())
	g.AddProcessEnv("HOSTNAME", g.Spec().Hostname)

	// Only add container environment variable if not already present
	foundContainerEnv := false
	for _, env := range g.Spec().Process.Env {
		if strings.HasPrefix(env, "container=") {
			foundContainerEnv = true
			break
		}
	}
	if !foundContainerEnv {
		g.AddProcessEnv("container", "libpod")
	}

	if c.runtime.config.CgroupManager == SystemdCgroupsManager {
		// When runc is set to use Systemd as a cgroup manager, it
		// expects cgroups to be passed as follows:
		// slice:prefix:name
		systemdCgroups := fmt.Sprintf("%s:libpod:%s", path.Base(c.config.CgroupParent), c.ID())
		logrus.Debugf("Setting CGroups for container %s to %s", c.ID(), systemdCgroups)
		g.SetLinuxCgroupsPath(systemdCgroups)
	} else {
		cgroupPath, err := c.CGroupPath()
		if err != nil {
			return nil, err
		}
		logrus.Debugf("Setting CGroup path for container %s to %s", c.ID(), cgroupPath)
		g.SetLinuxCgroupsPath(cgroupPath)
	}

	return g.Spec(), nil
}

// Add an existing container's namespace to the spec
func (c *Container) addNamespaceContainer(g *generate.Generator, ns LinuxNS, ctr string, specNS string) error {
	nsCtr, err := c.runtime.state.Container(ctr)
	if err != nil {
		return errors.Wrapf(err, "error retrieving dependency %s of container %s from state", ctr, c.ID())
	}

	// TODO need unlocked version of this for use in pods
	nsPath, err := nsCtr.NamespacePath(ns)
	if err != nil {
		return err
	}

	if err := g.AddOrReplaceLinuxNamespace(specNS, nsPath); err != nil {
		return err
	}

	return nil
}

func (c *Container) addImageVolumes(ctx context.Context, g *generate.Generator) error {
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
	for k := range imageData.ContainerConfig.Volumes {
		mount := spec.Mount{
			Destination: k,
			Type:        "bind",
			Options:     []string{"rbind", "rw"},
		}
		if MountExists(g.Mounts(), k) {
			continue
		}
		volumePath := filepath.Join(c.config.StaticDir, "volumes", k)
		if _, err := os.Stat(volumePath); os.IsNotExist(err) {
			if err = os.MkdirAll(volumePath, 0755); err != nil {
				return errors.Wrapf(err, "error creating directory %q for volume %q in container %q", volumePath, k, c.ID)
			}
			if err = label.Relabel(volumePath, c.config.MountLabel, false); err != nil {
				return errors.Wrapf(err, "error relabeling directory %q for volume %q in container %q", volumePath, k, c.ID)
			}
			srcPath := filepath.Join(mountPoint, k)
			if err = chrootarchive.NewArchiver(nil).CopyWithTar(srcPath, volumePath); err != nil && !os.IsNotExist(err) {
				return errors.Wrapf(err, "error populating directory %q for volume %q in container %q using contents of %q", volumePath, k, c.ID, srcPath)
			}
			mount.Source = volumePath
		}
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

func (c *Container) setupOCIHooks(ctx context.Context, g *generate.Generator) error {
	if c.runtime.config.HooksDir == "" {
		return nil
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
			return err
		}
	}

	manager, err := hooks.New(ctx, []string{c.runtime.config.HooksDir}, []string{}, lang)
	if err != nil {
		if c.runtime.config.HooksDirNotExistFatal || !os.IsNotExist(err) {
			return err
		}
		logrus.Warnf("failed to load hooks: {}", err)
		return nil
	}

	_, err = manager.Hooks(g.Spec(), c.Spec().Annotations, len(c.config.UserVolumes) > 0)
	return err
}
