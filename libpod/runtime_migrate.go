//go:build linux
// +build linux

package libpod

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/sirupsen/logrus"
)

func (r *Runtime) stopPauseProcess() error {
	if rootless.IsRootless() {
		pausePidPath, err := util.GetRootlessPauseProcessPidPath()
		if err != nil {
			return fmt.Errorf("could not get pause process pid file path: %w", err)
		}
		data, err := os.ReadFile(pausePidPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("cannot read pause process pid file: %w", err)
		}
		pausePid, err := strconv.Atoi(string(data))
		if err != nil {
			return fmt.Errorf("cannot parse pause pid file %s: %w", pausePidPath, err)
		}
		if err := os.Remove(pausePidPath); err != nil {
			return fmt.Errorf("cannot delete pause pid file %s: %w", pausePidPath, err)
		}
		if err := syscall.Kill(pausePid, syscall.SIGKILL); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) migrate() error {
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
	runtimeChangeRequested := r.migrateRuntime != ""
	requestedRuntime, runtimeExists := r.ociRuntimes[r.migrateRuntime]
	if !runtimeExists && runtimeChangeRequested {
		return fmt.Errorf("change to runtime %q requested but no such runtime is defined: %w", r.migrateRuntime, define.ErrInvalidArg)
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

		// Reset runtime
		if runtimeChangeRequested {
			logrus.Infof("Resetting container %s runtime to runtime %s", ctr.ID(), r.migrateRuntime)
			ctr.config.OCIRuntime = r.migrateRuntime
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
