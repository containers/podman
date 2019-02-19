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

	"github.com/containers/libpod/pkg/ctime"
	"github.com/containers/libpod/pkg/hooks"
	"github.com/containers/libpod/pkg/hooks/exec"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/mount"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/selinux/go-selinux/label"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"
	kwait "k8s.io/apimachinery/pkg/util/wait"
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

// exitFilePath gets the path to the container's exit file
func (c *Container) exitFilePath() string {
	return filepath.Join(c.runtime.ociRuntime.exitsDir, c.ID())
}

// Wait for the container's exit file to appear.
// When it does, update our state based on it.
func (c *Container) waitForExitFileAndSync() error {
	exitFile := c.exitFilePath()

	err := kwait.ExponentialBackoff(
		kwait.Backoff{
			Duration: 500 * time.Millisecond,
			Factor:   1.2,
			Steps:    6,
		},
		func() (bool, error) {
			_, err := os.Stat(exitFile)
			if err != nil {
				// wait longer
				return false, nil
			}
			return true, nil
		})
	if err != nil {
		// Exit file did not appear
		// Reset our state
		c.state.ExitCode = -1
		c.state.FinishedTime = time.Now()
		c.state.State = ContainerStateStopped

		if err2 := c.save(); err2 != nil {
			logrus.Errorf("Error saving container %s state: %v", c.ID(), err2)
		}

		return err
	}

	if err := c.runtime.ociRuntime.updateContainerStatus(c, false); err != nil {
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

	return nil
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
		if err := c.runtime.ociRuntime.updateContainerStatus(c, false); err != nil {
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
	span, _ := opentracing.StartSpanFromContext(ctx, "setupStorage")
	span.SetTag("type", "container")
	defer span.Finish()

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

	options := storage.ContainerOptions{
		IDMappingOptions: storage.IDMappingOptions{
			HostUIDMapping: true,
			HostGIDMapping: true,
		},
		LabelOpts: c.config.LabelOpts,
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
		defOptions, err := storage.GetDefaultMountOptions()
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

	if c.config.Rootfs == "" {
		options.IDMappingOptions = c.config.IDMappings
	}
	containerInfo, err := c.runtime.storageService.CreateContainerStorage(ctx, c.runtime.imageContext, c.config.RootfsImageName, c.config.RootfsImageID, c.config.Name, c.config.ID, options)
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

	c.config.ProcessLabel = containerInfo.ProcessLabel
	c.config.MountLabel = containerInfo.MountLabel
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
func resetState(state *ContainerState) error {
	state.PID = 0
	state.Mountpoint = ""
	state.Mounted = false
	if state.State != ContainerStateExited {
		state.State = ContainerStateConfigured
	}
	state.ExecSessions = make(map[string]*ExecSession)
	state.NetworkStatus = nil
	state.BindMounts = make(map[string]string)

	return nil
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

	// We need to pick up a new lock
	lock, err := c.runtime.lockManager.RetrieveLock(c.config.LockID)
	if err != nil {
		return errors.Wrapf(err, "error acquiring lock for container %s", c.ID())
	}
	c.lock = lock

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

	// Instead of outright deleting the exit file, rename it (if it exists).
	// We want to retain it so we can get the exit code of containers which
	// are removed (at least until we have a workable events system)
	exitFile := filepath.Join(c.runtime.ociRuntime.exitsDir, c.ID())
	oldExitFile := filepath.Join(c.runtime.ociRuntime.exitsDir, fmt.Sprintf("%s-old", c.ID()))
	if _, err := os.Stat(exitFile); err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrapf(err, "error running stat on container %s exit file", c.ID())
		}
	} else if err == nil {
		// Rename should replace the old exit file (if it exists)
		if err := os.Rename(exitFile, oldExitFile); err != nil {
			return errors.Wrapf(err, "error renaming container %s exit file", c.ID())
		}
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
	return (c.state.State != ContainerStateRunning && c.state.State != ContainerStatePaused), nil
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
func (c *Container) prepareToStart(ctx context.Context, recursive bool) (err error) {
	// Container must be created or stopped to be started
	if !(c.state.State == ContainerStateConfigured ||
		c.state.State == ContainerStateCreated ||
		c.state.State == ContainerStateStopped ||
		c.state.State == ContainerStateExited) {
		return errors.Wrapf(ErrCtrStateInvalid, "container %s must be in Created or Stopped state to be started", c.ID())
	}

	if !recursive {
		if err := c.checkDependenciesAndHandleError(ctx); err != nil {
			return err
		}
	} else {
		if err := c.startDependencies(ctx); err != nil {
			return err
		}
	}

	defer func() {
		if err != nil {
			if err2 := c.cleanup(ctx); err2 != nil {
				logrus.Errorf("error cleaning up container %s: %v", c.ID(), err2)
			}
		}
	}()

	if err := c.prepare(); err != nil {
		return err
	}

	if c.state.State == ContainerStateStopped {
		// Reinitialize the container if we need to
		if err := c.reinit(ctx); err != nil {
			return err
		}
	} else if c.state.State == ContainerStateConfigured ||
		c.state.State == ContainerStateExited {
		// Or initialize it if necessary
		if err := c.init(ctx); err != nil {
			return err
		}
	}
	return nil
}

// checks dependencies are running and prints a helpful message
func (c *Container) checkDependenciesAndHandleError(ctx context.Context) error {
	notRunning, err := c.checkDependenciesRunning()
	if err != nil {
		return errors.Wrapf(err, "error checking dependencies for container %s", c.ID())
	}
	if len(notRunning) > 0 {
		depString := strings.Join(notRunning, ",")
		return errors.Wrapf(ErrCtrStateInvalid, "some dependencies of container %s are not started: %s", c.ID(), depString)
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
	graph, err := buildContainerGraph(depCtrs)
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
		return errors.Wrapf(ErrNoSuchCtr, "All dependencies have dependencies of %s", c.ID())
	}

	ctrErrors := make(map[string]error)
	ctrsVisited := make(map[string]bool)

	// Traverse the graph beginning at nodes with no dependencies
	for _, node := range graph.noDepNodes {
		startNode(ctx, node, false, ctrErrors, ctrsVisited, true)
	}

	if len(ctrErrors) > 0 {
		logrus.Errorf("error starting some container dependencies")
		for _, e := range ctrErrors {
			logrus.Errorf("%q", e)
		}
		return errors.Wrapf(ErrInternal, "error starting some containers")
	}
	return nil
}

// getAllDependencies is a precursor to starting dependencies.
// To start a container with all of its dependencies, we need to recursively find all dependencies
// a container has, as well as each of those containers' dependencies, and so on
// To do so, keep track of containers already visisted (so there aren't redundant state lookups),
// and recursively search until we have reached the leafs of every dependency node.
// Since we need to start all dependencies for our original container to successfully start, we propegate any errors
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
			dep, err := c.runtime.state.LookupContainer(depID)
			if err != nil {
				return err
			}
			status, err := dep.State()
			if err != nil {
				return err
			}
			// if the dependency is already running, we can assume its dependencies are also running
			// so no need to add them to those we need to start
			if status != ContainerStateRunning {
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
	if c.config.NetMode == "slirp4netns" {
		return c.runtime.setupRootlessNetNS(c)
	}
	return c.runtime.setupNetNS(c)
}

// Initialize a container, creating it in the runtime
func (c *Container) init(ctx context.Context) error {
	span, _ := opentracing.StartSpanFromContext(ctx, "init")
	span.SetTag("struct", "container")
	defer span.Finish()

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
	if err := c.runtime.ociRuntime.createContainer(c, c.config.CgroupParent, nil); err != nil {
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
	span, _ := opentracing.StartSpanFromContext(ctx, "cleanupRuntime")
	span.SetTag("struct", "container")
	defer span.Finish()

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
	span, _ := opentracing.StartSpanFromContext(ctx, "reinit")
	span.SetTag("struct", "container")
	defer span.Finish()

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

	defer func() {
		if err != nil {
			if err2 := c.cleanup(ctx); err2 != nil {
				logrus.Errorf("error cleaning up container %s: %v", c.ID(), err2)
			}
		}
	}()

	if err := c.prepare(); err != nil {
		return err
	}

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
	if c.config.Spec.Process != nil {
		logrus.Debugf("Starting container %s with command %v", c.ID(), c.config.Spec.Process.Args)
	}

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

	// Wait until we have an exit file, and sync once we do
	return c.waitForExitFileAndSync()
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
	defer func() {
		if err != nil {
			if err2 := c.cleanup(ctx); err2 != nil {
				logrus.Errorf("error cleaning up container %s: %v", c.ID(), err2)
			}
		}
	}()
	if err := c.prepare(); err != nil {
		return err
	}

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
func (c *Container) mountStorage() (string, error) {
	var err error
	// Container already mounted, nothing to do
	if c.state.Mounted {
		return c.state.Mountpoint, nil
	}

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
	}

	// TODO: generalize this mount code so it will mount every mount in ctr.config.Mounts
	mountPoint := c.config.Rootfs
	if mountPoint == "" {
		mountPoint, err = c.mount()
		if err != nil {
			return "", err
		}
	}

	return mountPoint, nil
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

	span, _ := opentracing.StartSpanFromContext(ctx, "cleanup")
	span.SetTag("struct", "container")
	defer span.Finish()

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
	span, _ := opentracing.StartSpanFromContext(ctx, "delete")
	span.SetTag("struct", "container")
	defer span.Finish()

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
	span, _ := opentracing.StartSpanFromContext(ctx, "postDeleteHooks")
	span.SetTag("struct", "container")
	defer span.Finish()

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

// Warning: precreate hooks may alter 'config' in place.
func (c *Container) setupOCIHooks(ctx context.Context, config *spec.Spec) (extensionStageHooks map[string][]spec.Hook, err error) {
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
	if c.runtime.config.HooksDir == nil {
		if rootless.IsRootless() {
			return nil, nil
		}
		for _, hDir := range []string{hooks.DefaultDir, hooks.OverrideDir} {
			manager, err := hooks.New(ctx, []string{hDir}, []string{"precreate", "poststop"}, lang)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, err
			}
			hooks, err := manager.Hooks(config, c.Spec().Annotations, len(c.config.UserVolumes) > 0)
			if err != nil {
				return nil, err
			}
			if len(hooks) > 0 || config.Hooks != nil {
				logrus.Warnf("implicit hook directories are deprecated; set --hooks-dir=%q explicitly to continue to load hooks from this directory", hDir)
			}
			for i, hook := range hooks {
				allHooks[i] = hook
			}
		}
	} else {
		manager, err := hooks.New(ctx, c.runtime.config.HooksDir, []string{"precreate", "poststop"}, lang)
		if err != nil {
			if os.IsNotExist(err) {
				logrus.Warnf("Requested OCI hooks directory %q does not exist", c.runtime.config.HooksDir)
				return nil, nil
			}
			return nil, err
		}

		allHooks, err = manager.Hooks(config, c.Spec().Annotations, len(c.config.UserVolumes) > 0)
		if err != nil {
			return nil, err
		}
	}

	hookErr, err := exec.RuntimeConfigFilter(ctx, allHooks["precreate"], config, exec.DefaultPostKillTimeout)
	if err != nil {
		logrus.Warnf("container %s: precreate hook: %v", c.ID(), err)
		if hookErr != nil && hookErr != err {
			logrus.Debugf("container %s: precreate hook (hook error): %v", c.ID(), hookErr)
		}
		return nil, err
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

// namedVolumes returns named volumes for the container
func (c *Container) namedVolumes() ([]string, error) {
	var volumes []string
	for _, vol := range c.config.Spec.Mounts {
		if strings.HasPrefix(vol.Source, c.runtime.config.VolumePath) {
			volume := strings.TrimPrefix(vol.Source, c.runtime.config.VolumePath+"/")
			split := strings.Split(volume, "/")
			volume = split[0]
			if _, err := c.runtime.state.Volume(volume); err == nil {
				volumes = append(volumes, volume)
			}
		}
	}
	return volumes, nil
}

// this should be from chrootarchive.
func (c *Container) copyWithTarFromImage(src, dest string) error {
	mountpoint, err := c.mount()
	if err != nil {
		return err
	}
	a := archive.NewDefaultArchiver()
	source := filepath.Join(mountpoint, src)
	return a.CopyWithTar(source, dest)
}
