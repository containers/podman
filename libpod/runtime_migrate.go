//go:build !remote && (linux || freebsd)

package libpod

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/namespaces"
)

// Migrate stops the rootless pause process and performs any necessary database
// migrations that are required. It can also migrate all containers to a new OCI
// runtime, if requested.
func (r *Runtime) Migrate(newRuntime string) error {
	// Acquire the alive lock and hold it.
	// Ensures that we don't let other Podman commands run while we are
	// rewriting things in the DB.
	aliveLock, err := r.getRuntimeAliveLock()
	if err != nil {
		return fmt.Errorf("retrieving alive lock: %w", err)
	}
	aliveLock.Lock()
	defer aliveLock.Unlock()

	if !r.valid {
		return define.ErrRuntimeStopped
	}

	runningContainers, err := r.GetRunningContainers()
	if err != nil {
		return err
	}

	allCtrs, err := r.state.AllContainers(false)
	if err != nil {
		return err
	}

	logrus.Infof("Stopping all containers")
	for _, ctr := range runningContainers {
		fmt.Printf("stopped %s\n", ctr.ID())
		if err := ctr.Stop(); err != nil {
			return fmt.Errorf("cannot stop container %s: %w", ctr.ID(), err)
		}
	}

	// Did the user request a new runtime?
	runtimeChangeRequested := newRuntime != ""
	var requestedRuntime OCIRuntime
	if runtimeChangeRequested {
		runtime, exists := r.ociRuntimes[newRuntime]
		if !exists {
			return fmt.Errorf("change to runtime %q requested but no such runtime is defined: %w", newRuntime, define.ErrInvalidArg)
		}
		requestedRuntime = runtime
	}

	for _, ctr := range allCtrs {
		needsWrite := false

		// Reset pause process location
		oldLocation := filepath.Join(ctr.state.RunDir, "conmon.pid")
		if ctr.config.ConmonPidFile == oldLocation {
			logrus.Infof("Changing conmon PID file for %s", ctr.ID())
			ctr.config.ConmonPidFile = filepath.Join(ctr.config.StaticDir, "conmon.pid")
			needsWrite = true
		}

		// Migrate slirp4netns containers to pasta
		if ctr.config.NetMode == "slirp4netns" || strings.HasPrefix(string(ctr.config.NetMode), "slirp4netns:") {
			logrus.Infof("Migrating container %s from slirp4netns to pasta", ctr.ID())
			if opts, ok := ctr.config.NetworkOptions["slirp4netns"]; ok && len(opts) > 0 {
				logrus.Warnf("Container %s: dropping slirp4netns options %v; see podman-run(1) pasta section for equivalent options", ctr.ID(), opts)
			}
			ctr.config.NetMode = namespaces.NetworkMode("pasta")
			delete(ctr.config.NetworkOptions, "slirp4netns")
			needsWrite = true
		}

		// Reset runtime
		if runtimeChangeRequested && ctr.config.OCIRuntime != newRuntime {
			logrus.Infof("Resetting container %s runtime to runtime %s", ctr.ID(), newRuntime)
			ctr.config.OCIRuntime = newRuntime
			ctr.ociRuntime = requestedRuntime

			needsWrite = true
		}

		if needsWrite {
			if err := r.state.RewriteContainerConfig(ctr, ctr.config); err != nil {
				return fmt.Errorf("rewriting config for container %s: %w", ctr.ID(), err)
			}
		}
	}

	return r.stopPauseProcess()
}
