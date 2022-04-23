package libpod

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	metadata "github.com/checkpoint-restore/checkpointctl/lib"
	"github.com/containers/buildah/copier"
	"github.com/containers/buildah/pkg/overlay"
	butil "github.com/containers/buildah/util"
	"github.com/containers/common/libnetwork/etchosts"
	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/common/pkg/chown"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/pkg/ctime"
	"github.com/containers/podman/v4/pkg/hooks"
	"github.com/containers/podman/v4/pkg/hooks/exec"
	"github.com/containers/podman/v4/pkg/lookup"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/selinux"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/lockfile"
	"github.com/containers/storage/pkg/mount"
	"github.com/coreos/go-systemd/v22/daemon"
	securejoin "github.com/cyphar/filepath-securejoin"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const (
	// name of the directory holding the artifacts
	artifactsDir      = "artifacts"
	execDirPermission = 0755
	preCheckpointDir  = "pre-checkpoint"
)

// rootFsSize gets the size of the container's root filesystem
// A container FS is split into two parts.  The first is the top layer, a
// mutable layer, and the rest is the RootFS: the set of immutable layers
// that make up the image on which the container is based.
func (c *Container) rootFsSize() (int64, error) {
	if c.config.Rootfs != "" {
		return 0, nil
	}
	if c.runtime.store == nil {
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

// rwSize gets the size of the mutable top layer of the container.
func (c *Container) rwSize() (int64, error) {
	if c.config.Rootfs != "" {
		size, err := util.SizeOfPath(c.config.Rootfs)
		return int64(size), err
	}

	container, err := c.runtime.store.Container(c.ID())
	if err != nil {
		return 0, err
	}

	// The top layer of a container is
	// the only readable/writeable layer, all others are immutable.
	rwLayer, err := c.runtime.store.Layer(container.LayerID)
	if err != nil {
		return 0, err
	}

	// Get the size of the top layer by calculating the size of the diff
	// between the layer and its parent.
	return c.runtime.store.DiffSize(rwLayer.Parent, rwLayer.ID)
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

// CheckpointVolumesPath returns the path to the directory containing the checkpointed volumes
func (c *Container) CheckpointVolumesPath() string {
	return filepath.Join(c.bundlePath(), metadata.CheckpointVolumesDirectory)
}

// CheckpointPath returns the path to the directory containing the checkpoint
func (c *Container) CheckpointPath() string {
	return filepath.Join(c.bundlePath(), metadata.CheckpointDirectory)
}

// PreCheckpointPath returns the path to the directory containing the pre-checkpoint-images
func (c *Container) PreCheckPointPath() string {
	return filepath.Join(c.bundlePath(), preCheckpointDir)
}

// AttachSocketPath retrieves the path of the container's attach socket
func (c *Container) AttachSocketPath() (string, error) {
	return c.ociRuntime.AttachSocketPath(c)
}

// exitFilePath gets the path to the container's exit file
func (c *Container) exitFilePath() (string, error) {
	return c.ociRuntime.ExitFilePath(c)
}

// Wait for the container's exit file to appear.
// When it does, update our state based on it.
func (c *Container) waitForExitFileAndSync() error {
	exitFile, err := c.exitFilePath()
	if err != nil {
		return err
	}

	chWait := make(chan error)
	defer close(chWait)

	_, err = WaitForFile(exitFile, chWait, time.Second*5)
	if err != nil {
		// Exit file did not appear
		// Reset our state
		c.state.ExitCode = -1
		c.state.FinishedTime = time.Now()
		c.state.State = define.ContainerStateStopped

		if err2 := c.save(); err2 != nil {
			logrus.Errorf("Saving container %s state: %v", c.ID(), err2)
		}

		return err
	}

	if err := c.checkExitFile(); err != nil {
		return err
	}

	return c.save()
}

// Handle the container exit file.
// The exit file is used to supply container exit time and exit code.
// This assumes the exit file already exists.
func (c *Container) handleExitFile(exitFile string, fi os.FileInfo) error {
	c.state.FinishedTime = ctime.Created(fi)
	statusCodeStr, err := ioutil.ReadFile(exitFile)
	if err != nil {
		return errors.Wrapf(err, "failed to read exit file for container %s", c.ID())
	}
	statusCode, err := strconv.Atoi(string(statusCodeStr))
	if err != nil {
		return errors.Wrapf(err, "error converting exit status code (%q) for container %s to int",
			c.ID(), statusCodeStr)
	}
	c.state.ExitCode = int32(statusCode)

	oomFilePath := filepath.Join(c.bundlePath(), "oom")
	if _, err = os.Stat(oomFilePath); err == nil {
		c.state.OOMKilled = true
	}

	c.state.Exited = true

	// Write an event for the container's death
	c.newContainerExitedEvent(c.state.ExitCode)

	return nil
}

func (c *Container) shouldRestart() bool {
	// If we did not get a restart policy match, return false
	// Do the same if we're not a policy that restarts.
	if !c.state.RestartPolicyMatch ||
		c.config.RestartPolicy == define.RestartPolicyNo ||
		c.config.RestartPolicy == define.RestartPolicyNone {
		return false
	}

	// If we're RestartPolicyOnFailure, we need to check retries and exit
	// code.
	if c.config.RestartPolicy == define.RestartPolicyOnFailure {
		if c.state.ExitCode == 0 {
			return false
		}

		// If we don't have a max retries set, continue
		if c.config.RestartRetries > 0 {
			if c.state.RestartCount >= c.config.RestartRetries {
				return false
			}
		}
	}
	return true
}

// Handle container restart policy.
// This is called when a container has exited, and was not explicitly stopped by
// an API call to stop the container or pod it is in.
func (c *Container) handleRestartPolicy(ctx context.Context) (_ bool, retErr error) {
	if !c.shouldRestart() {
		return false, nil
	}
	logrus.Debugf("Restarting container %s due to restart policy %s", c.ID(), c.config.RestartPolicy)

	// Need to check if dependencies are alive.
	if err := c.checkDependenciesAndHandleError(); err != nil {
		return false, err
	}

	// Is the container running again?
	// If so, we don't have to do anything
	if c.ensureState(define.ContainerStateRunning, define.ContainerStatePaused) {
		return false, nil
	} else if c.state.State == define.ContainerStateUnknown {
		return false, errors.Wrapf(define.ErrInternal, "invalid container state encountered in restart attempt")
	}

	c.newContainerEvent(events.Restart)

	// Increment restart count
	c.state.RestartCount++
	logrus.Debugf("Container %s now on retry %d", c.ID(), c.state.RestartCount)
	if err := c.save(); err != nil {
		return false, err
	}

	defer func() {
		if retErr != nil {
			if err := c.cleanup(ctx); err != nil {
				logrus.Errorf("Cleaning up container %s: %v", c.ID(), err)
			}
		}
	}()
	if err := c.prepare(); err != nil {
		return false, err
	}

	// setup slirp4netns again because slirp4netns will die when conmon exits
	if c.config.NetMode.IsSlirp4netns() {
		err := c.runtime.setupSlirp4netns(c, c.state.NetNS)
		if err != nil {
			return false, err
		}
	}

	// setup rootlesskit port forwarder again since it dies when conmon exits
	// we use rootlesskit port forwarder only as rootless and when bridge network is used
	if rootless.IsRootless() && c.config.NetMode.IsBridge() && len(c.config.PortMappings) > 0 {
		err := c.runtime.setupRootlessPortMappingViaRLK(c, c.state.NetNS.Path(), c.state.NetworkStatus)
		if err != nil {
			return false, err
		}
	}

	if c.state.State == define.ContainerStateStopped {
		// Reinitialize the container if we need to
		if err := c.reinit(ctx, true); err != nil {
			return false, err
		}
	} else if c.ensureState(define.ContainerStateConfigured, define.ContainerStateExited) {
		// Initialize the container
		if err := c.init(ctx, true); err != nil {
			return false, err
		}
	}
	if err := c.start(); err != nil {
		return false, err
	}
	return true, nil
}

// Ensure that the container is in a specific state or state.
// Returns true if the container is in one of the given states,
// or false otherwise.
func (c *Container) ensureState(states ...define.ContainerStatus) bool {
	for _, state := range states {
		if state == c.state.State {
			return true
		}
	}
	return false
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
	if c.ensureState(define.ContainerStateCreated, define.ContainerStateRunning, define.ContainerStateStopped, define.ContainerStatePaused) {
		oldState := c.state.State

		if err := c.checkExitFile(); err != nil {
			return err
		}

		// Only save back to DB if state changed
		if c.state.State != oldState {
			// Check for a restart policy match
			if c.config.RestartPolicy != define.RestartPolicyNone && c.config.RestartPolicy != define.RestartPolicyNo &&
				(oldState == define.ContainerStateRunning || oldState == define.ContainerStatePaused) &&
				(c.state.State == define.ContainerStateStopped || c.state.State == define.ContainerStateExited) &&
				!c.state.StoppedByUser {
				c.state.RestartPolicyMatch = true
			}

			if err := c.save(); err != nil {
				return err
			}
		}
	}

	if !c.valid {
		return errors.Wrapf(define.ErrCtrRemoved, "container %s is not valid", c.ID())
	}

	return nil
}

func (c *Container) setupStorageMapping(dest, from *storage.IDMappingOptions) {
	if c.config.Rootfs != "" {
		return
	}
	*dest = *from
	// If we are creating a container inside a pod, we always want to inherit the
	// userns settings from the infra container. So clear the auto userns settings
	// so that we don't request storage for a new uid/gid map.
	if c.PodID() != "" && !c.IsInfra() {
		dest.AutoUserNs = false
	}
	if dest.AutoUserNs {
		overrides := c.getUserOverrides()
		dest.AutoUserNsOpts.PasswdFile = overrides.ContainerEtcPasswdPath
		dest.AutoUserNsOpts.GroupFile = overrides.ContainerEtcGroupPath
		if c.config.User != "" {
			initialSize := uint32(0)
			parts := strings.Split(c.config.User, ":")
			for _, p := range parts {
				s, err := strconv.ParseUint(p, 10, 32)
				if err == nil && uint32(s) > initialSize {
					initialSize = uint32(s)
				}
			}
			dest.AutoUserNsOpts.InitialSize = initialSize + 1
		}
	} else if c.config.Spec.Linux != nil {
		dest.UIDMap = nil
		for _, r := range c.config.Spec.Linux.UIDMappings {
			u := idtools.IDMap{
				ContainerID: int(r.ContainerID),
				HostID:      int(r.HostID),
				Size:        int(r.Size),
			}
			dest.UIDMap = append(dest.UIDMap, u)
		}
		dest.GIDMap = nil
		for _, r := range c.config.Spec.Linux.GIDMappings {
			g := idtools.IDMap{
				ContainerID: int(r.ContainerID),
				HostID:      int(r.HostID),
				Size:        int(r.Size),
			}
			dest.GIDMap = append(dest.GIDMap, g)
		}
		dest.HostUIDMapping = false
		dest.HostGIDMapping = false
	}
}

// Create container root filesystem for use
func (c *Container) setupStorage(ctx context.Context) error {
	if !c.valid {
		return errors.Wrapf(define.ErrCtrRemoved, "container %s is not valid", c.ID())
	}

	if c.state.State != define.ContainerStateConfigured {
		return errors.Wrapf(define.ErrCtrStateInvalid, "container %s must be in Configured state to have storage set up", c.ID())
	}

	// Need both an image ID and image name, plus a bool telling us whether to use the image configuration
	if c.config.Rootfs == "" && (c.config.RootfsImageID == "" || c.config.RootfsImageName == "") {
		return errors.Wrapf(define.ErrInvalidArg, "must provide image ID and image name to use an image")
	}
	options := storage.ContainerOptions{
		IDMappingOptions: storage.IDMappingOptions{
			HostUIDMapping: true,
			HostGIDMapping: true,
		},
		LabelOpts: c.config.LabelOpts,
	}

	options.StorageOpt = c.config.StorageOpts

	if c.restoreFromCheckpoint && c.config.ProcessLabel != "" && c.config.MountLabel != "" {
		// If restoring from a checkpoint, the root file-system needs
		// to be mounted with the same SELinux labels as it was mounted
		// previously. But only if both labels have been set. For
		// privileged containers or '--ipc host' only ProcessLabel will
		// be set and so we will skip it for cases like that.
		if options.Flags == nil {
			options.Flags = make(map[string]interface{})
		}
		options.Flags["ProcessLabel"] = c.config.ProcessLabel
		options.Flags["MountLabel"] = c.config.MountLabel
	}
	if c.config.Privileged {
		privOpt := func(opt string) bool {
			for _, privopt := range []string{"nodev", "nosuid", "noexec"} {
				if opt == privopt {
					return true
				}
			}
			return false
		}

		defOptions, err := storage.GetMountOptions(c.runtime.store.GraphDriverName(), c.runtime.store.GraphOptions())
		if err != nil {
			return errors.Wrapf(err, "error getting default mount options")
		}
		var newOptions []string
		for _, opt := range defOptions {
			if !privOpt(opt) {
				newOptions = append(newOptions, opt)
			}
		}
		options.MountOpts = newOptions
	}

	options.Volatile = c.config.Volatile

	c.setupStorageMapping(&options.IDMappingOptions, &c.config.IDMappings)

	// Unless the user has specified a name, use a randomly generated one.
	// Note that name conflicts may occur (see #11735), so we need to loop.
	generateName := c.config.Name == ""
	var containerInfo ContainerInfo
	var containerInfoErr error
	for {
		if generateName {
			name, err := c.runtime.generateName()
			if err != nil {
				return err
			}
			c.config.Name = name
		}
		containerInfo, containerInfoErr = c.runtime.storageService.CreateContainerStorage(ctx, c.runtime.imageContext, c.config.RootfsImageName, c.config.RootfsImageID, c.config.Name, c.config.ID, options)

		if !generateName || errors.Cause(containerInfoErr) != storage.ErrDuplicateName {
			break
		}
	}
	if containerInfoErr != nil {
		return errors.Wrapf(containerInfoErr, "error creating container storage")
	}

	// only reconfig IDMappings if layer was mounted from storage
	// if its a external overlay do not reset IDmappings
	if !c.config.RootfsOverlay {
		c.config.IDMappings.UIDMap = containerInfo.UIDMap
		c.config.IDMappings.GIDMap = containerInfo.GIDMap
	}

	processLabel, err := c.processLabel(containerInfo.ProcessLabel)
	if err != nil {
		return err
	}
	c.config.ProcessLabel = processLabel
	c.config.MountLabel = containerInfo.MountLabel
	c.config.StaticDir = containerInfo.Dir
	c.state.RunDir = containerInfo.RunDir

	if len(c.config.IDMappings.UIDMap) != 0 || len(c.config.IDMappings.GIDMap) != 0 {
		if err := os.Chown(containerInfo.RunDir, c.RootUID(), c.RootGID()); err != nil {
			return err
		}

		if err := os.Chown(containerInfo.Dir, c.RootUID(), c.RootGID()); err != nil {
			return err
		}
	}

	// Set the default Entrypoint and Command
	if containerInfo.Config != nil {
		// Set CMD in the container to the default configuration only if ENTRYPOINT is not set by the user.
		if c.config.Entrypoint == nil && c.config.Command == nil {
			c.config.Command = containerInfo.Config.Config.Cmd
		}
		if c.config.Entrypoint == nil {
			c.config.Entrypoint = containerInfo.Config.Config.Entrypoint
		}
	}

	artifacts := filepath.Join(c.config.StaticDir, artifactsDir)
	if err := os.MkdirAll(artifacts, 0755); err != nil {
		return errors.Wrap(err, "error creating artifacts directory")
	}

	return nil
}

func (c *Container) processLabel(processLabel string) (string, error) {
	if !c.Systemd() && !c.ociRuntime.SupportsKVM() {
		return processLabel, nil
	}
	ctrSpec, err := c.specFromState()
	if err != nil {
		return "", err
	}
	label, ok := ctrSpec.Annotations[define.InspectAnnotationLabel]
	if !ok || !strings.Contains(label, "type:") {
		switch {
		case c.ociRuntime.SupportsKVM():
			return selinux.KVMLabel(processLabel)
		case c.Systemd():
			return selinux.InitLabel(processLabel)
		}
	}
	return processLabel, nil
}

// Tear down a container's storage prior to removal
func (c *Container) teardownStorage() error {
	if c.ensureState(define.ContainerStateRunning, define.ContainerStatePaused) {
		return errors.Wrapf(define.ErrCtrStateInvalid, "cannot remove storage for container %s as it is running or paused", c.ID())
	}

	artifacts := filepath.Join(c.config.StaticDir, artifactsDir)
	if err := os.RemoveAll(artifacts); err != nil {
		return errors.Wrapf(err, "error removing container %s artifacts %q", c.ID(), artifacts)
	}

	if err := c.cleanupStorage(); err != nil {
		return errors.Wrapf(err, "failed to cleanup container %s storage", c.ID())
	}

	if err := c.runtime.storageService.DeleteContainer(c.ID()); err != nil {
		// If the container has already been removed, warn but do not
		// error - we wanted it gone, it is already gone.
		// Potentially another tool using containers/storage already
		// removed it?
		if errors.Cause(err) == storage.ErrNotAContainer || errors.Cause(err) == storage.ErrContainerUnknown {
			logrus.Infof("Storage for container %s already removed", c.ID())
			return nil
		}

		return errors.Wrapf(err, "error removing container %s root filesystem", c.ID())
	}

	return nil
}

// Reset resets state fields to default values.
// It is performed before a refresh and clears the state after a reboot.
// It does not save the results - assumes the database will do that for us.
func resetState(state *ContainerState) {
	state.PID = 0
	state.ConmonPID = 0
	state.Mountpoint = ""
	state.Mounted = false
	if state.State != define.ContainerStateExited {
		state.State = define.ContainerStateConfigured
	}
	state.ExecSessions = make(map[string]*ExecSession)
	state.LegacyExecSessions = nil
	state.BindMounts = make(map[string]string)
	state.StoppedByUser = false
	state.RestartPolicyMatch = false
	state.RestartCount = 0
	state.Checkpointed = false
	state.Restored = false
	state.CheckpointedTime = time.Time{}
	state.RestoredTime = time.Time{}
	state.CheckpointPath = ""
	state.CheckpointLog = ""
	state.RestoreLog = ""
}

// Refresh refreshes the container's state after a restart.
// Refresh cannot perform any operations that would lock another container.
// We cannot guarantee any other container has a valid lock at the time it is
// running.
func (c *Container) refresh() error {
	// Don't need a full sync, but we do need to update from the database to
	// pick up potentially-missing container state
	if err := c.runtime.state.UpdateContainer(c); err != nil {
		return err
	}

	if !c.valid {
		return errors.Wrapf(define.ErrCtrRemoved, "container %s is not valid - may have been removed", c.ID())
	}

	// We need to get the container's temporary directory from c/storage
	// It was lost in the reboot and must be recreated
	dir, err := c.runtime.storageService.GetRunDir(c.ID())
	if err != nil {
		return errors.Wrapf(err, "error retrieving temporary directory for container %s", c.ID())
	}
	c.state.RunDir = dir

	if len(c.config.IDMappings.UIDMap) != 0 || len(c.config.IDMappings.GIDMap) != 0 {
		info, err := os.Stat(c.runtime.config.Engine.TmpDir)
		if err != nil {
			return err
		}
		if err := os.Chmod(c.runtime.config.Engine.TmpDir, info.Mode()|0111); err != nil {
			return err
		}
		root := filepath.Join(c.runtime.config.Engine.TmpDir, "containers-root", c.ID())
		if err := os.MkdirAll(root, 0755); err != nil {
			return errors.Wrapf(err, "error creating userNS tmpdir for container %s", c.ID())
		}
		if err := os.Chown(root, c.RootUID(), c.RootGID()); err != nil {
			return err
		}
	}

	// We need to pick up a new lock
	lock, err := c.runtime.lockManager.AllocateAndRetrieveLock(c.config.LockID)
	if err != nil {
		return errors.Wrapf(err, "error acquiring lock %d for container %s", c.config.LockID, c.ID())
	}
	c.lock = lock

	c.state.NetworkStatus = nil
	c.state.NetworkStatusOld = nil

	// Rewrite the config if necessary.
	// Podman 4.0 uses a new port format in the config.
	// getContainerConfigFromDB() already converted the old ports to the new one
	// but it did not write the config to the db back for performance reasons.
	// If a rewrite must happen the config.rewrite field is set to true.
	if c.config.rewrite {
		// SafeRewriteContainerConfig must be used with care. Make sure to not change config fields by accident.
		if err := c.runtime.state.SafeRewriteContainerConfig(c, "", "", c.config); err != nil {
			return errors.Wrapf(err, "failed to rewrite the config for container %s", c.config.ID)
		}
		c.config.rewrite = false
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
	attachFile, err := c.AttachSocketPath()
	if err != nil {
		return errors.Wrapf(err, "failed to get attach socket path for container %s", c.ID())
	}

	if err := os.Remove(attachFile); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "error removing container %s attach file", c.ID())
	}

	ctlFile := filepath.Join(c.bundlePath(), "ctl")
	if err := os.Remove(ctlFile); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "error removing container %s ctl file", c.ID())
	}

	winszFile := filepath.Join(c.bundlePath(), "winsz")
	if err := os.Remove(winszFile); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "error removing container %s winsz file", c.ID())
	}

	oomFile := filepath.Join(c.bundlePath(), "oom")
	if err := os.Remove(oomFile); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "error removing container %s OOM file", c.ID())
	}

	// Remove the exit file so we don't leak memory in tmpfs
	exitFile, err := c.exitFilePath()
	if err != nil {
		return err
	}
	if err := os.Remove(exitFile); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "error removing container %s exit file", c.ID())
	}

	return nil
}

func (c *Container) export(path string) error {
	mountPoint := c.state.Mountpoint
	if !c.state.Mounted {
		containerMount, err := c.runtime.store.Mount(c.ID(), c.config.MountLabel)
		if err != nil {
			return errors.Wrapf(err, "mounting container %q", c.ID())
		}
		mountPoint = containerMount
		defer func() {
			if _, err := c.runtime.store.Unmount(c.ID(), false); err != nil {
				logrus.Errorf("Unmounting container %q: %v", c.ID(), err)
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
func (c *Container) isStopped() (bool, int32, error) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()
	}
	err := c.syncContainer()
	if err != nil {
		return true, -1, err
	}

	return !c.ensureState(define.ContainerStateRunning, define.ContainerStatePaused, define.ContainerStateStopping), c.state.ExitCode, nil
}

// save container state to the database
func (c *Container) save() error {
	if err := c.runtime.state.SaveContainer(c); err != nil {
		return errors.Wrapf(err, "error saving container %s state", c.ID())
	}
	return nil
}

// Checks the container is in the right state, then initializes the container in preparation to start the container.
// If recursive is true, each of the containers dependencies will be started.
// Otherwise, this function will return with error if there are dependencies of this container that aren't running.
func (c *Container) prepareToStart(ctx context.Context, recursive bool) (retErr error) {
	// Container must be created or stopped to be started
	if !c.ensureState(define.ContainerStateConfigured, define.ContainerStateCreated, define.ContainerStateStopped, define.ContainerStateExited) {
		return errors.Wrapf(define.ErrCtrStateInvalid, "container %s must be in Created or Stopped state to be started", c.ID())
	}

	if !recursive {
		if err := c.checkDependenciesAndHandleError(); err != nil {
			return err
		}
	} else {
		if err := c.startDependencies(ctx); err != nil {
			return err
		}
	}

	defer func() {
		if retErr != nil {
			if err := c.cleanup(ctx); err != nil {
				logrus.Errorf("Cleaning up container %s: %v", c.ID(), err)
			}
		}
	}()

	if err := c.prepare(); err != nil {
		return err
	}

	if c.state.State == define.ContainerStateStopped {
		// Reinitialize the container if we need to
		if err := c.reinit(ctx, false); err != nil {
			return err
		}
	} else if c.ensureState(define.ContainerStateConfigured, define.ContainerStateExited) {
		// Or initialize it if necessary
		if err := c.init(ctx, false); err != nil {
			return err
		}
	}
	return nil
}

// checks dependencies are running and prints a helpful message
func (c *Container) checkDependenciesAndHandleError() error {
	notRunning, err := c.checkDependenciesRunning()
	if err != nil {
		return errors.Wrapf(err, "error checking dependencies for container %s", c.ID())
	}
	if len(notRunning) > 0 {
		depString := strings.Join(notRunning, ",")
		return errors.Wrapf(define.ErrCtrStateInvalid, "some dependencies of container %s are not started: %s", c.ID(), depString)
	}

	return nil
}

// Recursively start all dependencies of a container so the container can be started.
func (c *Container) startDependencies(ctx context.Context) error {
	depCtrIDs := c.Dependencies()
	if len(depCtrIDs) == 0 {
		return nil
	}

	depVisitedCtrs := make(map[string]*Container)
	if err := c.getAllDependencies(depVisitedCtrs); err != nil {
		return errors.Wrapf(err, "error starting dependency for container %s", c.ID())
	}

	// Because of how Go handles passing slices through functions, a slice cannot grow between function calls
	// without clunky syntax. Circumnavigate this by translating the map to a slice for buildContainerGraph
	depCtrs := make([]*Container, 0)
	for _, ctr := range depVisitedCtrs {
		depCtrs = append(depCtrs, ctr)
	}

	// Build a dependency graph of containers
	graph, err := BuildContainerGraph(depCtrs)
	if err != nil {
		return errors.Wrapf(err, "error generating dependency graph for container %s", c.ID())
	}

	// If there are no containers without dependencies, we can't start
	// Error out
	if len(graph.noDepNodes) == 0 {
		// we have no dependencies that need starting, go ahead and return
		if len(graph.nodes) == 0 {
			return nil
		}
		return errors.Wrapf(define.ErrNoSuchCtr, "All dependencies have dependencies of %s", c.ID())
	}

	ctrErrors := make(map[string]error)
	ctrsVisited := make(map[string]bool)

	// Traverse the graph beginning at nodes with no dependencies
	for _, node := range graph.noDepNodes {
		startNode(ctx, node, false, ctrErrors, ctrsVisited, true)
	}

	if len(ctrErrors) > 0 {
		logrus.Errorf("Starting some container dependencies")
		for _, e := range ctrErrors {
			logrus.Errorf("%q", e)
		}
		return errors.Wrapf(define.ErrInternal, "error starting some containers")
	}
	return nil
}

// getAllDependencies is a precursor to starting dependencies.
// To start a container with all of its dependencies, we need to recursively find all dependencies
// a container has, as well as each of those containers' dependencies, and so on
// To do so, keep track of containers already visited (so there aren't redundant state lookups),
// and recursively search until we have reached the leafs of every dependency node.
// Since we need to start all dependencies for our original container to successfully start, we propagate any errors
// in looking up dependencies.
// Note: this function is currently meant as a robust solution to a narrow problem: start an infra-container when
// a container in the pod is run. It has not been tested for performance past one level, so expansion of recursive start
// must be tested first.
func (c *Container) getAllDependencies(visited map[string]*Container) error {
	depIDs := c.Dependencies()
	if len(depIDs) == 0 {
		return nil
	}
	for _, depID := range depIDs {
		if _, ok := visited[depID]; !ok {
			dep, err := c.runtime.state.Container(depID)
			if err != nil {
				return err
			}
			status, err := dep.State()
			if err != nil {
				return err
			}
			// if the dependency is already running, we can assume its dependencies are also running
			// so no need to add them to those we need to start
			if status != define.ContainerStateRunning {
				visited[depID] = dep
				if err := dep.getAllDependencies(visited); err != nil {
					return err
				}
			}
		}
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
		if state != define.ContainerStateRunning && !depCtr.config.IsInfra {
			notRunning = append(notRunning, dep)
		}
		depCtrs[dep] = depCtr
	}

	return notRunning, nil
}

func (c *Container) completeNetworkSetup() error {
	var outResolvConf []string
	netDisabled, err := c.NetworkDisabled()
	if err != nil {
		return err
	}
	if !c.config.PostConfigureNetNS || netDisabled {
		return nil
	}
	if err := c.syncContainer(); err != nil {
		return err
	}
	if err := c.runtime.setupNetNS(c); err != nil {
		return err
	}
	state := c.state
	// collect any dns servers that cni tells us to use (dnsname)
	for _, status := range c.getNetworkStatus() {
		for _, server := range status.DNSServerIPs {
			outResolvConf = append(outResolvConf, fmt.Sprintf("nameserver %s", server))
		}
	}
	// check if we have a bindmount for /etc/hosts
	if hostsBindMount, ok := state.BindMounts[config.DefaultHostsFile]; ok {
		entries, err := c.getHostsEntries()
		if err != nil {
			return err
		}
		// add new container ips to the hosts file
		if err := etchosts.Add(hostsBindMount, entries); err != nil {
			return err
		}
	}

	// check if we have a bindmount for resolv.conf
	resolvBindMount := state.BindMounts["/etc/resolv.conf"]
	if len(outResolvConf) < 1 || resolvBindMount == "" || len(c.config.NetNsCtr) > 0 {
		return nil
	}
	// read the existing resolv.conf
	b, err := ioutil.ReadFile(resolvBindMount)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(b), "\n") {
		// only keep things that don't start with nameserver from the old
		// resolv.conf file
		if !strings.HasPrefix(line, "nameserver") {
			outResolvConf = append([]string{line}, outResolvConf...)
		}
	}
	// write and return
	return ioutil.WriteFile(resolvBindMount, []byte(strings.Join(outResolvConf, "\n")), 0644)
}

// Initialize a container, creating it in the runtime
func (c *Container) init(ctx context.Context, retainRetries bool) error {
	// Unconditionally remove conmon temporary files.
	// We've been running into far too many issues where they block startup.
	if err := c.removeConmonFiles(); err != nil {
		return err
	}

	// Generate the OCI newSpec
	newSpec, err := c.generateSpec(ctx)
	if err != nil {
		return err
	}

	// Make sure the workdir exists while initializing container
	if err := c.resolveWorkDir(); err != nil {
		return err
	}

	// Save the OCI newSpec to disk
	if err := c.saveSpec(newSpec); err != nil {
		return err
	}

	for _, v := range c.config.NamedVolumes {
		if err := c.fixVolumePermissions(v); err != nil {
			return err
		}
	}

	// With the spec complete, do an OCI create
	if _, err = c.ociRuntime.CreateContainer(c, nil); err != nil {
		return err
	}

	logrus.Debugf("Created container %s in OCI runtime", c.ID())

	// Remove any exec sessions leftover from a potential prior run.
	if len(c.state.ExecSessions) > 0 {
		if err := c.runtime.state.RemoveContainerExecSessions(c); err != nil {
			logrus.Errorf("Removing container %s exec sessions from DB: %v", c.ID(), err)
		}
		c.state.ExecSessions = make(map[string]*ExecSession)
	}

	c.state.Checkpointed = false
	c.state.Restored = false
	c.state.CheckpointedTime = time.Time{}
	c.state.RestoredTime = time.Time{}
	c.state.CheckpointPath = ""
	c.state.CheckpointLog = ""
	c.state.RestoreLog = ""
	c.state.ExitCode = 0
	c.state.Exited = false
	c.state.State = define.ContainerStateCreated
	c.state.StoppedByUser = false
	c.state.RestartPolicyMatch = false

	if !retainRetries {
		c.state.RestartCount = 0
	}

	if err := c.save(); err != nil {
		return err
	}
	if c.config.HealthCheckConfig != nil {
		if err := c.createTimer(); err != nil {
			logrus.Error(err)
		}
	}

	defer c.newContainerEvent(events.Init)
	return c.completeNetworkSetup()
}

// Clean up a container in the OCI runtime.
// Deletes the container in the runtime, and resets its state to Exited.
// The container can be restarted cleanly after this.
func (c *Container) cleanupRuntime(ctx context.Context) error {
	// If the container is not ContainerStateStopped or
	// ContainerStateCreated, do nothing.
	if !c.ensureState(define.ContainerStateStopped, define.ContainerStateCreated) {
		return nil
	}

	// If necessary, delete attach and ctl files
	if err := c.removeConmonFiles(); err != nil {
		return err
	}

	if err := c.delete(ctx); err != nil {
		return err
	}

	// If we were Stopped, we are now Exited, as we've removed ourself
	// from the runtime.
	// If we were Created, we are now Configured.
	if c.state.State == define.ContainerStateStopped {
		c.state.State = define.ContainerStateExited
	} else if c.state.State == define.ContainerStateCreated {
		c.state.State = define.ContainerStateConfigured
	}

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
func (c *Container) reinit(ctx context.Context, retainRetries bool) error {
	logrus.Debugf("Recreating container %s in OCI runtime", c.ID())

	if err := c.cleanupRuntime(ctx); err != nil {
		return err
	}

	// Initialize the container again
	return c.init(ctx, retainRetries)
}

// Initialize (if necessary) and start a container
// Performs all necessary steps to start a container that is not running
// Does not lock or check validity
func (c *Container) initAndStart(ctx context.Context) (retErr error) {
	// If we are ContainerStateUnknown, throw an error
	if c.state.State == define.ContainerStateUnknown {
		return errors.Wrapf(define.ErrCtrStateInvalid, "container %s is in an unknown state", c.ID())
	} else if c.state.State == define.ContainerStateRemoving {
		return errors.Wrapf(define.ErrCtrStateInvalid, "cannot start container %s as it is being removed", c.ID())
	}

	// If we are running, do nothing
	if c.state.State == define.ContainerStateRunning {
		return nil
	}
	// If we are paused, throw an error
	if c.state.State == define.ContainerStatePaused {
		return errors.Wrapf(define.ErrCtrStateInvalid, "cannot start paused container %s", c.ID())
	}

	defer func() {
		if retErr != nil {
			if err := c.cleanup(ctx); err != nil {
				logrus.Errorf("Cleaning up container %s: %v", c.ID(), err)
			}
		}
	}()

	if err := c.prepare(); err != nil {
		return err
	}

	// If we are ContainerStateStopped we need to remove from runtime
	// And reset to ContainerStateConfigured
	if c.state.State == define.ContainerStateStopped {
		logrus.Debugf("Recreating container %s in OCI runtime", c.ID())

		if err := c.reinit(ctx, false); err != nil {
			return err
		}
	} else if c.ensureState(define.ContainerStateConfigured, define.ContainerStateExited) {
		if err := c.init(ctx, false); err != nil {
			return err
		}
	}

	// Now start the container
	return c.start()
}

// Internal, non-locking function to start a container
func (c *Container) start() error {
	if c.config.Spec.Process != nil {
		logrus.Debugf("Starting container %s with command %v", c.ID(), c.config.Spec.Process.Args)
	}

	if err := c.ociRuntime.StartContainer(c); err != nil {
		return err
	}
	logrus.Debugf("Started container %s", c.ID())

	c.state.State = define.ContainerStateRunning

	if c.config.SdNotifyMode != define.SdNotifyModeIgnore {
		payload := fmt.Sprintf("MAINPID=%d", c.state.ConmonPID)
		if c.config.SdNotifyMode == define.SdNotifyModeConmon {
			payload += "\n"
			payload += daemon.SdNotifyReady
		}
		if sent, err := daemon.SdNotify(false, payload); err != nil {
			logrus.Errorf("Notifying systemd of Conmon PID: %s", err.Error())
		} else if sent {
			logrus.Debugf("Notify sent successfully")
		}
	}

	// Check if healthcheck is not nil and --no-healthcheck option is not set.
	// If --no-healthcheck is set Test will be always set to `[NONE]` so no need
	// to update status in such case.
	if c.config.HealthCheckConfig != nil && !(len(c.config.HealthCheckConfig.Test) == 1 && c.config.HealthCheckConfig.Test[0] == "NONE") {
		if err := c.updateHealthStatus(define.HealthCheckStarting); err != nil {
			logrus.Error(err)
		}
		if err := c.startTimer(); err != nil {
			logrus.Error(err)
		}
	}

	defer c.newContainerEvent(events.Start)

	return c.save()
}

// Internal, non-locking function to stop container
func (c *Container) stop(timeout uint) error {
	logrus.Debugf("Stopping ctr %s (timeout %d)", c.ID(), timeout)

	// If the container is running in a PID Namespace, then killing the
	// primary pid is enough to kill the container.  If it is not running in
	// a pid namespace then the OCI Runtime needs to kill ALL processes in
	// the containers cgroup in order to make sure the container is stopped.
	all := !c.hasNamespace(spec.PIDNamespace)
	// We can't use --all if Cgroups aren't present.
	// Rootless containers with Cgroups v1 and NoCgroups are both cases
	// where this can happen.
	if all {
		if c.config.NoCgroups {
			all = false
		} else if rootless.IsRootless() {
			// Only do this check if we need to
			unified, err := cgroups.IsCgroup2UnifiedMode()
			if err != nil {
				return err
			}
			if !unified {
				all = false
			}
		}
	}

	// Check if conmon is still alive.
	// If it is not, we won't be getting an exit file.
	conmonAlive, err := c.ociRuntime.CheckConmonRunning(c)
	if err != nil {
		return err
	}

	// Set the container state to "stopping" and unlock the container
	// before handing it over to conmon to unblock other commands.  #8501
	// demonstrates nicely that a high stop timeout will block even simple
	// commands such as `podman ps` from progressing if the container lock
	// is held when busy-waiting for the container to be stopped.
	c.state.State = define.ContainerStateStopping
	if err := c.save(); err != nil {
		return errors.Wrapf(err, "error saving container %s state before stopping", c.ID())
	}
	if !c.batched {
		c.lock.Unlock()
	}

	stopErr := c.ociRuntime.StopContainer(c, timeout, all)

	if !c.batched {
		c.lock.Lock()
		if err := c.syncContainer(); err != nil {
			switch errors.Cause(err) {
			// If the container has already been removed (e.g., via
			// the cleanup process), there's nothing left to do.
			case define.ErrNoSuchCtr, define.ErrCtrRemoved:
				return stopErr
			default:
				if stopErr != nil {
					logrus.Errorf("Syncing container %s status: %v", c.ID(), err)
					return stopErr
				}
				return err
			}
		}
	}

	// We have to check stopErr *after* we lock again - otherwise, we have a
	// change of panicking on a double-unlock. Ref: GH Issue 9615
	if stopErr != nil {
		return stopErr
	}

	// Since we're now subject to a race condition with other processes who
	// may have altered the state (and other data), let's check if the
	// state has changed.  If so, we should return immediately and log a
	// warning.
	if c.state.State != define.ContainerStateStopping {
		logrus.Warnf(
			"Container %q state changed from %q to %q while waiting for it to be stopped: discontinuing stop procedure as another process interfered",
			c.ID(), define.ContainerStateStopping, c.state.State,
		)
		return nil
	}

	c.newContainerEvent(events.Stop)

	c.state.PID = 0
	c.state.ConmonPID = 0
	c.state.StoppedByUser = true

	if !conmonAlive {
		// Conmon is dead, so we can't expect an exit code.
		c.state.ExitCode = -1
		c.state.FinishedTime = time.Now()
		c.state.State = define.ContainerStateStopped
		if err := c.save(); err != nil {
			logrus.Errorf("Saving container %s status: %v", c.ID(), err)
		}

		return errors.Wrapf(define.ErrConmonDead, "container %s conmon process missing, cannot retrieve exit code", c.ID())
	}

	if err := c.save(); err != nil {
		return errors.Wrapf(err, "error saving container %s state after stopping", c.ID())
	}

	// Wait until we have an exit file, and sync once we do
	if err := c.waitForExitFileAndSync(); err != nil {
		return err
	}

	return nil
}

// Internal, non-locking function to pause a container
func (c *Container) pause() error {
	if c.config.NoCgroups {
		return errors.Wrapf(define.ErrNoCgroups, "cannot pause without using Cgroups")
	}

	if rootless.IsRootless() {
		cgroupv2, err := cgroups.IsCgroup2UnifiedMode()
		if err != nil {
			return errors.Wrap(err, "failed to determine cgroupversion")
		}
		if !cgroupv2 {
			return errors.Wrap(define.ErrNoCgroups, "can not pause containers on rootless containers with cgroup V1")
		}
	}

	if err := c.ociRuntime.PauseContainer(c); err != nil {
		// TODO when using docker-py there is some sort of race/incompatibility here
		return err
	}

	logrus.Debugf("Paused container %s", c.ID())

	c.state.State = define.ContainerStatePaused

	return c.save()
}

// Internal, non-locking function to unpause a container
func (c *Container) unpause() error {
	if c.config.NoCgroups {
		return errors.Wrapf(define.ErrNoCgroups, "cannot unpause without using Cgroups")
	}

	if err := c.ociRuntime.UnpauseContainer(c); err != nil {
		// TODO when using docker-py there is some sort of race/incompatibility here
		return err
	}

	logrus.Debugf("Unpaused container %s", c.ID())

	c.state.State = define.ContainerStateRunning

	return c.save()
}

// Internal, non-locking function to restart a container
func (c *Container) restartWithTimeout(ctx context.Context, timeout uint) (retErr error) {
	if !c.ensureState(define.ContainerStateConfigured, define.ContainerStateCreated, define.ContainerStateRunning, define.ContainerStateStopped, define.ContainerStateExited) {
		return errors.Wrapf(define.ErrCtrStateInvalid, "unable to restart a container in a paused or unknown state")
	}

	c.newContainerEvent(events.Restart)

	if c.state.State == define.ContainerStateRunning {
		conmonPID := c.state.ConmonPID
		if err := c.stop(timeout); err != nil {
			return err
		}
		// Old versions of conmon have a bug where they create the exit file before
		// closing open file descriptors causing a race condition when restarting
		// containers with open ports since we cannot bind the ports as they're not
		// yet closed by conmon.
		//
		// Killing the old conmon PID is ~okay since it forces the FDs of old conmons
		// to be closed, while it's a NOP for newer versions which should have
		// exited already.
		if conmonPID != 0 {
			// Ignore errors from FindProcess() as conmon could already have exited.
			p, err := os.FindProcess(conmonPID)
			if p != nil && err == nil {
				if err = p.Kill(); err != nil {
					logrus.Debugf("error killing conmon process: %v", err)
				}
			}
		}
		// Ensure we tear down the container network so it will be
		// recreated - otherwise, behavior of restart differs from stop
		// and start
		if err := c.cleanupNetwork(); err != nil {
			return err
		}
	}
	defer func() {
		if retErr != nil {
			if err := c.cleanup(ctx); err != nil {
				logrus.Errorf("Cleaning up container %s: %v", c.ID(), err)
			}
		}
	}()
	if err := c.prepare(); err != nil {
		return err
	}

	if c.state.State == define.ContainerStateStopped {
		// Reinitialize the container if we need to
		if err := c.reinit(ctx, false); err != nil {
			return err
		}
	} else if c.state.State == define.ContainerStateConfigured ||
		c.state.State == define.ContainerStateExited {
		// Initialize the container
		if err := c.init(ctx, false); err != nil {
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
func (c *Container) mountStorage() (_ string, deferredErr error) {
	var err error
	// Container already mounted, nothing to do
	if c.state.Mounted {
		return c.state.Mountpoint, nil
	}

	if !c.config.NoShm {
		mounted, err := mount.Mounted(c.config.ShmDir)
		if err != nil {
			return "", errors.Wrapf(err, "unable to determine if %q is mounted", c.config.ShmDir)
		}

		if !mounted && !MountExists(c.config.Spec.Mounts, "/dev/shm") {
			shmOptions := fmt.Sprintf("mode=1777,size=%d", c.config.ShmSize)
			if err := c.mountSHM(shmOptions); err != nil {
				return "", err
			}
			if err := os.Chown(c.config.ShmDir, c.RootUID(), c.RootGID()); err != nil {
				return "", errors.Wrapf(err, "failed to chown %s", c.config.ShmDir)
			}
			defer func() {
				if deferredErr != nil {
					if err := c.unmountSHM(c.config.ShmDir); err != nil {
						logrus.Errorf("Unmounting SHM for container %s after mount error: %v", c.ID(), err)
					}
				}
			}()
		}
	}

	// We need to mount the container before volumes - to ensure the copyup
	// works properly.
	mountPoint := c.config.Rootfs
	// Check if overlay has to be created on top of Rootfs
	if c.config.RootfsOverlay {
		overlayDest := c.runtime.RunRoot()
		contentDir, err := overlay.GenerateStructure(overlayDest, c.ID(), "rootfs", c.RootUID(), c.RootGID())
		if err != nil {
			return "", errors.Wrapf(err, "rootfs-overlay: failed to create TempDir in the %s directory", overlayDest)
		}
		overlayMount, err := overlay.Mount(contentDir, c.config.Rootfs, overlayDest, c.RootUID(), c.RootGID(), c.runtime.store.GraphOptions())
		if err != nil {
			return "", errors.Wrapf(err, "rootfs-overlay: creating overlay failed %q", c.config.Rootfs)
		}

		// Seems fuse-overlayfs is not present
		// fallback to native overlay
		if overlayMount.Type == "overlay" {
			overlayMount.Options = append(overlayMount.Options, "nodev")
			mountOpts := label.FormatMountLabel(strings.Join(overlayMount.Options, ","), c.MountLabel())
			err = mount.Mount("overlay", overlayMount.Source, overlayMount.Type, mountOpts)
			if err != nil {
				return "", errors.Wrapf(err, "rootfs-overlay: creating overlay failed %q from native overlay", c.config.Rootfs)
			}
		}

		mountPoint = overlayMount.Source
		execUser, err := lookup.GetUserGroupInfo(mountPoint, c.config.User, nil)
		if err != nil {
			return "", err
		}
		hostUID, hostGID, err := butil.GetHostIDs(util.IDtoolsToRuntimeSpec(c.config.IDMappings.UIDMap), util.IDtoolsToRuntimeSpec(c.config.IDMappings.GIDMap), uint32(execUser.Uid), uint32(execUser.Gid))
		if err != nil {
			return "", errors.Wrap(err, "unable to get host UID and host GID")
		}

		//note: this should not be recursive, if using external rootfs users should be responsible on configuring ownership.
		if err := chown.ChangeHostPathOwnership(mountPoint, false, int(hostUID), int(hostGID)); err != nil {
			return "", err
		}
	}

	if mountPoint == "" {
		mountPoint, err = c.mount()
		if err != nil {
			return "", err
		}
		defer func() {
			if deferredErr != nil {
				if err := c.unmount(false); err != nil {
					logrus.Errorf("Unmounting container %s after mount error: %v", c.ID(), err)
				}
			}
		}()
	}

	rootUID, rootGID := c.RootUID(), c.RootGID()

	dirfd, err := unix.Open(mountPoint, unix.O_RDONLY|unix.O_PATH, 0)
	if err != nil {
		return "", errors.Wrap(err, "open mount point")
	}
	defer unix.Close(dirfd)

	err = unix.Mkdirat(dirfd, "etc", 0755)
	if err != nil && !os.IsExist(err) {
		return "", errors.Wrap(err, "create /etc")
	}
	// If the etc directory was created, chown it to root in the container
	if err == nil && (rootUID != 0 || rootGID != 0) {
		err = unix.Fchownat(dirfd, "etc", rootUID, rootGID, unix.AT_SYMLINK_NOFOLLOW)
		if err != nil {
			return "", errors.Wrap(err, "chown /etc")
		}
	}

	etcInTheContainerPath, err := securejoin.SecureJoin(mountPoint, "etc")
	if err != nil {
		return "", errors.Wrap(err, "resolve /etc in the container")
	}

	etcInTheContainerFd, err := unix.Open(etcInTheContainerPath, unix.O_RDONLY|unix.O_PATH, 0)
	if err != nil {
		return "", errors.Wrap(err, "open /etc in the container")
	}
	defer unix.Close(etcInTheContainerFd)

	// If /etc/mtab does not exist in container image, then we need to
	// create it, so that mount command within the container will work.
	err = unix.Symlinkat("/proc/mounts", etcInTheContainerFd, "mtab")
	if err != nil && !os.IsExist(err) {
		return "", errors.Wrap(err, "creating /etc/mtab symlink")
	}
	// If the symlink was created, then also chown it to root in the container
	if err == nil && (rootUID != 0 || rootGID != 0) {
		err = unix.Fchownat(etcInTheContainerFd, "mtab", rootUID, rootGID, unix.AT_SYMLINK_NOFOLLOW)
		if err != nil {
			return "", errors.Wrap(err, "chown /etc/mtab")
		}
	}

	// Request a mount of all named volumes
	for _, v := range c.config.NamedVolumes {
		vol, err := c.mountNamedVolume(v, mountPoint)
		if err != nil {
			return "", err
		}
		defer func() {
			if deferredErr == nil {
				return
			}
			vol.lock.Lock()
			if err := vol.unmount(false); err != nil {
				logrus.Errorf("Unmounting volume %s after error mounting container %s: %v", vol.Name(), c.ID(), err)
			}
			vol.lock.Unlock()
		}()
	}

	return mountPoint, nil
}

// Mount a single named volume into the container.
// If necessary, copy up image contents into the volume.
// Does not verify that the name volume given is actually present in container
// config.
// Returns the volume that was mounted.
func (c *Container) mountNamedVolume(v *ContainerNamedVolume, mountpoint string) (*Volume, error) {
	logrus.Debugf("Going to mount named volume %s", v.Name)
	vol, err := c.runtime.state.Volume(v.Name)
	if err != nil {
		return nil, errors.Wrapf(err, "error retrieving named volume %s for container %s", v.Name, c.ID())
	}

	if vol.config.LockID == c.config.LockID {
		return nil, errors.Wrapf(define.ErrWillDeadlock, "container %s and volume %s share lock ID %d", c.ID(), vol.Name(), c.config.LockID)
	}
	vol.lock.Lock()
	defer vol.lock.Unlock()
	if vol.needsMount() {
		if err := vol.mount(); err != nil {
			return nil, errors.Wrapf(err, "error mounting volume %s for container %s", vol.Name(), c.ID())
		}
	}
	// The volume may need a copy-up. Check the state.
	if err := vol.update(); err != nil {
		return nil, err
	}
	if vol.state.NeedsCopyUp {
		logrus.Debugf("Copying up contents from container %s to volume %s", c.ID(), vol.Name())

		// If the volume is not empty, we should not copy up.
		volMount := vol.mountPoint()
		contents, err := ioutil.ReadDir(volMount)
		if err != nil {
			return nil, errors.Wrapf(err, "error listing contents of volume %s mountpoint when copying up from container %s", vol.Name(), c.ID())
		}
		if len(contents) > 0 {
			// The volume is not empty. It was likely modified
			// outside of Podman. For safety, let's not copy up into
			// it. Fixes CVE-2020-1726.
			return vol, nil
		}

		srcDir, err := securejoin.SecureJoin(mountpoint, v.Dest)
		if err != nil {
			return nil, errors.Wrapf(err, "error calculating destination path to copy up container %s volume %s", c.ID(), vol.Name())
		}
		// Do a manual stat on the source directory to verify existence.
		// Skip the rest if it exists.
		// TODO: Should this be stat or lstat? I'm using lstat because I
		// think copy-up doesn't happen when the source is a link.
		srcStat, err := os.Lstat(srcDir)
		if err != nil {
			if os.IsNotExist(err) {
				// Source does not exist, don't bother copying
				// up.
				return vol, nil
			}
			return nil, errors.Wrapf(err, "error identifying source directory for copy up into volume %s", vol.Name())
		}
		// If it's not a directory we're mounting over it.
		if !srcStat.IsDir() {
			return vol, nil
		}
		// Read contents, do not bother continuing if it's empty. Fixes
		// a bizarre issue where something copier.Get will ENOENT on
		// empty directories and sometimes it will not.
		// RHBZ#1928643
		srcContents, err := ioutil.ReadDir(srcDir)
		if err != nil {
			return nil, errors.Wrapf(err, "error reading contents of source directory for copy up into volume %s", vol.Name())
		}
		if len(srcContents) == 0 {
			return vol, nil
		}

		// Set NeedsCopyUp to false since we are about to do first copy
		// Do not copy second time.
		vol.state.NeedsCopyUp = false
		if err := vol.save(); err != nil {
			return nil, err
		}

		// Buildah Copier accepts a reader, so we'll need a pipe.
		reader, writer := io.Pipe()
		defer reader.Close()

		errChan := make(chan error, 1)

		logrus.Infof("About to copy up into volume %s", vol.Name())

		// Copy, container side: get a tar archive of what needs to be
		// streamed into the volume.
		go func() {
			defer writer.Close()
			getOptions := copier.GetOptions{
				KeepDirectoryNames: false,
			}
			errChan <- copier.Get(srcDir, "", getOptions, []string{"/."}, writer)
		}()

		// Copy, volume side: stream what we've written to the pipe, into
		// the volume.
		copyOpts := copier.PutOptions{}
		if err := copier.Put(volMount, "", copyOpts, reader); err != nil {
			err2 := <-errChan
			if err2 != nil {
				logrus.Errorf("Streaming contents of container %s directory for volume copy-up: %v", c.ID(), err2)
			}
			return nil, errors.Wrapf(err, "error copying up to volume %s", vol.Name())
		}

		if err := <-errChan; err != nil {
			return nil, errors.Wrapf(err, "error streaming container content for copy up into volume %s", vol.Name())
		}
	}
	return vol, nil
}

// cleanupStorage unmounts and cleans up the container's root filesystem
func (c *Container) cleanupStorage() error {
	if !c.state.Mounted {
		// Already unmounted, do nothing
		logrus.Debugf("Container %s storage is already unmounted, skipping...", c.ID())
		return nil
	}

	var cleanupErr error

	markUnmounted := func() {
		c.state.Mountpoint = ""
		c.state.Mounted = false

		if c.valid {
			if err := c.save(); err != nil {
				if cleanupErr != nil {
					logrus.Errorf("Unmounting container %s: %v", c.ID(), cleanupErr)
				}
				cleanupErr = err
			}
		}
	}

	// umount rootfs overlay if it was created
	if c.config.RootfsOverlay {
		overlayBasePath := filepath.Dir(c.state.Mountpoint)
		if err := overlay.Unmount(overlayBasePath); err != nil {
			if cleanupErr != nil {
				logrus.Errorf("Failed to cleanup overlay mounts for %s: %v", c.ID(), err)
			}
			cleanupErr = err
		}
	}

	for _, containerMount := range c.config.Mounts {
		if err := c.unmountSHM(containerMount); err != nil {
			if cleanupErr != nil {
				logrus.Errorf("Unmounting container %s: %v", c.ID(), cleanupErr)
			}
			cleanupErr = err
		}
	}

	if err := c.cleanupOverlayMounts(); err != nil {
		// If the container can't remove content report the error
		logrus.Errorf("Failed to cleanup overlay mounts for %s: %v", c.ID(), err)
		cleanupErr = err
	}

	if c.config.Rootfs != "" {
		markUnmounted()
		return cleanupErr
	}

	if err := c.unmount(false); err != nil {
		// If the container has already been removed, warn but don't
		// error
		// We still want to be able to kick the container out of the
		// state
		if errors.Cause(err) == storage.ErrNotAContainer || errors.Cause(err) == storage.ErrContainerUnknown || errors.Cause(err) == storage.ErrLayerNotMounted {
			logrus.Errorf("Storage for container %s has been removed", c.ID())
		} else {
			if cleanupErr != nil {
				logrus.Errorf("Cleaning up container %s storage: %v", c.ID(), cleanupErr)
			}
			cleanupErr = err
		}
	}

	// Request an unmount of all named volumes
	for _, v := range c.config.NamedVolumes {
		vol, err := c.runtime.state.Volume(v.Name)
		if err != nil {
			if cleanupErr != nil {
				logrus.Errorf("Unmounting container %s: %v", c.ID(), cleanupErr)
			}
			cleanupErr = errors.Wrapf(err, "error retrieving named volume %s for container %s", v.Name, c.ID())

			// We need to try and unmount every volume, so continue
			// if they fail.
			continue
		}

		if vol.needsMount() {
			vol.lock.Lock()
			if err := vol.unmount(false); err != nil {
				if cleanupErr != nil {
					logrus.Errorf("Unmounting container %s: %v", c.ID(), cleanupErr)
				}
				cleanupErr = errors.Wrapf(err, "error unmounting volume %s for container %s", vol.Name(), c.ID())
			}
			vol.lock.Unlock()
		}
	}

	markUnmounted()
	return cleanupErr
}

// Unmount the container and free its resources
func (c *Container) cleanup(ctx context.Context) error {
	var lastError error

	logrus.Debugf("Cleaning up container %s", c.ID())

	// Remove healthcheck unit/timer file if it execs
	if c.config.HealthCheckConfig != nil {
		if err := c.removeTransientFiles(ctx); err != nil {
			logrus.Errorf("Removing timer for container %s healthcheck: %v", c.ID(), err)
		}
	}

	// Clean up network namespace, if present
	if err := c.cleanupNetwork(); err != nil {
		lastError = errors.Wrapf(err, "error removing container %s network", c.ID())
	}

	// cleanup host entry if it is shared
	if c.config.NetNsCtr != "" {
		if hoststFile, ok := c.state.BindMounts[config.DefaultHostsFile]; ok {
			if _, err := os.Stat(hoststFile); err == nil {
				// we cannot use the dependency container lock due ABBA deadlocks
				if lock, err := lockfile.GetLockfile(hoststFile); err == nil {
					lock.Lock()
					// make sure to ignore ENOENT error in case the netns container was cleanup before this one
					if err := etchosts.Remove(hoststFile, getLocalhostHostEntry(c)); err != nil && !errors.Is(err, os.ErrNotExist) {
						// this error is not fatal we still want to do proper cleanup
						logrus.Errorf("failed to remove hosts entry from the netns containers /etc/hosts: %v", err)
					}
					lock.Unlock()
				}
			}
		}
	}

	// Remove the container from the runtime, if necessary.
	// Do this *before* unmounting storage - some runtimes (e.g. Kata)
	// apparently object to having storage removed while the container still
	// exists.
	if err := c.cleanupRuntime(ctx); err != nil {
		if lastError != nil {
			logrus.Errorf("Removing container %s from OCI runtime: %v", c.ID(), err)
		} else {
			lastError = err
		}
	}

	// Unmount storage
	if err := c.cleanupStorage(); err != nil {
		if lastError != nil {
			logrus.Errorf("Unmounting container %s storage: %v", c.ID(), err)
		} else {
			lastError = errors.Wrapf(err, "error unmounting container %s storage", c.ID())
		}
	}

	// Unmount image volumes
	for _, v := range c.config.ImageVolumes {
		img, _, err := c.runtime.LibimageRuntime().LookupImage(v.Source, nil)
		if err != nil {
			if lastError == nil {
				lastError = err
				continue
			}
			logrus.Errorf("Unmounting image volume %q:%q :%v", v.Source, v.Dest, err)
		}
		if err := img.Unmount(false); err != nil {
			if lastError == nil {
				lastError = err
				continue
			}
			logrus.Errorf("Unmounting image volume %q:%q :%v", v.Source, v.Dest, err)
		}
	}

	return lastError
}

// delete deletes the container and runs any configured poststop
// hooks.
func (c *Container) delete(ctx context.Context) error {
	if err := c.ociRuntime.DeleteContainer(c); err != nil {
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
func (c *Container) postDeleteHooks(ctx context.Context) error {
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
				hook := hook
				logrus.Debugf("container %s: invoke poststop hook %d, path %s", c.ID(), i, hook.Path)
				var stderr, stdout bytes.Buffer
				hookErr, err := exec.Run(ctx, &hook, state, &stdout, &stderr, exec.DefaultPostKillTimeout)
				if err != nil {
					logrus.Warnf("Container %s: poststop hook %d: %v", c.ID(), i, err)
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

// writeStringToRundir writes the given string to a file with the given name in
// the container's temporary files directory. The file will be chown'd to the
// container's root user and have an appropriate SELinux label set.
// If a file with the same name already exists, it will be deleted and recreated
// with the new contents.
// Returns the full path to the new file.
func (c *Container) writeStringToRundir(destFile, contents string) (string, error) {
	destFileName := filepath.Join(c.state.RunDir, destFile)

	if err := os.Remove(destFileName); err != nil && !os.IsNotExist(err) {
		return "", errors.Wrapf(err, "error removing %s for container %s", destFile, c.ID())
	}

	if err := writeStringToPath(destFileName, contents, c.config.MountLabel, c.RootUID(), c.RootGID()); err != nil {
		return "", err
	}

	return destFileName, nil
}

// writeStringToStaticDir writes the given string to a file with the given name
// in the container's permanent files directory. The file will be chown'd to the
// container's root user and have an appropriate SELinux label set.
// Unlike writeStringToRundir, will *not* delete and re-create if the file
// already exists (will instead error).
// Returns the full path to the new file.
func (c *Container) writeStringToStaticDir(filename, contents string) (string, error) {
	destFileName := filepath.Join(c.config.StaticDir, filename)

	if err := writeStringToPath(destFileName, contents, c.config.MountLabel, c.RootUID(), c.RootGID()); err != nil {
		return "", err
	}

	return destFileName, nil
}

// saveSpec saves the OCI spec to disk, replacing any existing specs for the container
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

// Warning: precreate hooks may alter 'config' in place.
func (c *Container) setupOCIHooks(ctx context.Context, config *spec.Spec) (map[string][]spec.Hook, error) {
	allHooks := make(map[string][]spec.Hook)
	if c.runtime.config.Engine.HooksDir == nil {
		if rootless.IsRootless() {
			return nil, nil
		}
		for _, hDir := range []string{hooks.DefaultDir, hooks.OverrideDir} {
			manager, err := hooks.New(ctx, []string{hDir}, []string{"precreate", "poststop"})
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, err
			}
			ociHooks, err := manager.Hooks(config, c.config.Spec.Annotations, len(c.config.UserVolumes) > 0)
			if err != nil {
				return nil, err
			}
			if len(ociHooks) > 0 || config.Hooks != nil {
				logrus.Warnf("Implicit hook directories are deprecated; set --ociHooks-dir=%q explicitly to continue to load ociHooks from this directory", hDir)
			}
			for i, hook := range ociHooks {
				allHooks[i] = hook
			}
		}
	} else {
		manager, err := hooks.New(ctx, c.runtime.config.Engine.HooksDir, []string{"precreate", "poststop"})
		if err != nil {
			return nil, err
		}

		allHooks, err = manager.Hooks(config, c.config.Spec.Annotations, len(c.config.UserVolumes) > 0)
		if err != nil {
			return nil, err
		}
	}

	hookErr, err := exec.RuntimeConfigFilter(ctx, allHooks["precreate"], config, exec.DefaultPostKillTimeout)
	if err != nil {
		logrus.Warnf("Container %s: precreate hook: %v", c.ID(), err)
		if hookErr != nil && hookErr != err {
			logrus.Debugf("container %s: precreate hook (hook error): %v", c.ID(), hookErr)
		}
		return nil, err
	}

	return allHooks, nil
}

// mount mounts the container's root filesystem
func (c *Container) mount() (string, error) {
	if c.state.State == define.ContainerStateRemoving {
		return "", errors.Wrapf(define.ErrCtrStateInvalid, "cannot mount container %s as it is being removed", c.ID())
	}

	mountPoint, err := c.runtime.storageService.MountContainerImage(c.ID())
	if err != nil {
		return "", errors.Wrapf(err, "error mounting storage for container %s", c.ID())
	}
	mountPoint, err = filepath.EvalSymlinks(mountPoint)
	if err != nil {
		return "", errors.Wrapf(err, "error resolving storage path for container %s", c.ID())
	}
	if err := os.Chown(mountPoint, c.RootUID(), c.RootGID()); err != nil {
		return "", errors.Wrapf(err, "cannot chown %s to %d:%d", mountPoint, c.RootUID(), c.RootGID())
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

// checkReadyForRemoval checks whether the given container is ready to be
// removed.
// These checks are only used if force-remove is not specified.
// If it is, we'll remove the container anyways.
// Returns nil if safe to remove, or an error describing why it's unsafe if not.
func (c *Container) checkReadyForRemoval() error {
	if c.state.State == define.ContainerStateUnknown {
		return errors.Wrapf(define.ErrCtrStateInvalid, "container %s is in invalid state", c.ID())
	}

	if c.ensureState(define.ContainerStateRunning, define.ContainerStatePaused) && !c.IsInfra() {
		return errors.Wrapf(define.ErrCtrStateInvalid, "cannot remove container %s as it is %s - running or paused containers cannot be removed without force", c.ID(), c.state.State.String())
	}

	// Check exec sessions
	sessions, err := c.getActiveExecSessions()
	if err != nil {
		return err
	}
	if len(sessions) != 0 {
		return errors.Wrapf(define.ErrCtrStateInvalid, "cannot remove container %s as it has active exec sessions", c.ID())
	}

	return nil
}

// canWithPrevious return the stat of the preCheckPoint dir
func (c *Container) canWithPrevious() error {
	_, err := os.Stat(c.PreCheckPointPath())
	return err
}

// prepareCheckpointExport writes the config and spec to
// JSON files for later export
func (c *Container) prepareCheckpointExport() error {
	networks, err := c.networks()
	if err != nil {
		return err
	}
	// make sure to exclude the short ID alias since the container gets a new ID on restore
	for net, opts := range networks {
		newAliases := make([]string, 0, len(opts.Aliases))
		for _, alias := range opts.Aliases {
			if alias != c.config.ID[:12] {
				newAliases = append(newAliases, alias)
			}
		}
		opts.Aliases = newAliases
		networks[net] = opts
	}

	// add the networks from the db to the config so that the exported checkpoint still stores all current networks
	c.config.Networks = networks
	// save live config
	if _, err := metadata.WriteJSONFile(c.config, c.bundlePath(), metadata.ConfigDumpFile); err != nil {
		return err
	}

	// save spec
	jsonPath := filepath.Join(c.bundlePath(), "config.json")
	g, err := generate.NewFromFile(jsonPath)
	if err != nil {
		logrus.Debugf("generating spec for container %q failed with %v", c.ID(), err)
		return err
	}
	if _, err := metadata.WriteJSONFile(g.Config, c.bundlePath(), metadata.SpecDumpFile); err != nil {
		return err
	}

	return nil
}

// SortUserVolumes sorts the volumes specified for a container
// between named and normal volumes
func (c *Container) SortUserVolumes(ctrSpec *spec.Spec) ([]*ContainerNamedVolume, []spec.Mount) {
	namedUserVolumes := []*ContainerNamedVolume{}
	userMounts := []spec.Mount{}

	// We need to parse all named volumes and mounts into maps, so we don't
	// end up with repeated lookups for each user volume.
	// Map destination to struct, as destination is what is stored in
	// UserVolumes.
	namedVolumes := make(map[string]*ContainerNamedVolume)
	mounts := make(map[string]spec.Mount)
	for _, namedVol := range c.config.NamedVolumes {
		namedVolumes[namedVol.Dest] = namedVol
	}
	for _, mount := range ctrSpec.Mounts {
		mounts[mount.Destination] = mount
	}

	for _, vol := range c.config.UserVolumes {
		if volume, ok := namedVolumes[vol]; ok {
			namedUserVolumes = append(namedUserVolumes, volume)
		} else if mount, ok := mounts[vol]; ok {
			userMounts = append(userMounts, mount)
		} else {
			logrus.Warnf("Could not find mount at destination %q when parsing user volumes for container %s", vol, c.ID())
		}
	}
	return namedUserVolumes, userMounts
}

// Check for an exit file, and handle one if present
func (c *Container) checkExitFile() error {
	// If the container's not running, nothing to do.
	if !c.ensureState(define.ContainerStateRunning, define.ContainerStatePaused, define.ContainerStateStopping) {
		return nil
	}

	exitFile, err := c.exitFilePath()
	if err != nil {
		return err
	}

	// Check for the exit file
	info, err := os.Stat(exitFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Container is still running, no error
			return nil
		}

		return errors.Wrapf(err, "error running stat on container %s exit file", c.ID())
	}

	// Alright, it exists. Transition to Stopped state.
	c.state.State = define.ContainerStateStopped
	c.state.PID = 0
	c.state.ConmonPID = 0

	// Read the exit file to get our stopped time and exit code.
	return c.handleExitFile(exitFile, info)
}

func (c *Container) hasNamespace(namespace spec.LinuxNamespaceType) bool {
	if c.config.Spec == nil || c.config.Spec.Linux == nil {
		return false
	}
	for _, n := range c.config.Spec.Linux.Namespaces {
		if n.Type == namespace {
			return true
		}
	}
	return false
}

// extractSecretToStorage copies a secret's data from the secrets manager to the container's static dir
func (c *Container) extractSecretToCtrStorage(secr *ContainerSecret) error {
	manager, err := c.runtime.SecretsManager()
	if err != nil {
		return err
	}
	_, data, err := manager.LookupSecretData(secr.Name)
	if err != nil {
		return err
	}
	secretFile := filepath.Join(c.config.SecretsPath, secr.Name)

	hostUID, hostGID, err := butil.GetHostIDs(util.IDtoolsToRuntimeSpec(c.config.IDMappings.UIDMap), util.IDtoolsToRuntimeSpec(c.config.IDMappings.GIDMap), secr.UID, secr.GID)
	if err != nil {
		return errors.Wrap(err, "unable to extract secret")
	}
	err = ioutil.WriteFile(secretFile, data, 0644)
	if err != nil {
		return errors.Wrapf(err, "unable to create %s", secretFile)
	}
	if err := os.Lchown(secretFile, int(hostUID), int(hostGID)); err != nil {
		return err
	}
	if err := os.Chmod(secretFile, os.FileMode(secr.Mode)); err != nil {
		return err
	}
	if err := label.Relabel(secretFile, c.config.MountLabel, false); err != nil {
		return err
	}
	return nil
}
