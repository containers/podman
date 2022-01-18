// +build linux

package libpod

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (r *Runtime) stopPauseProcess() error {
	if rootless.IsRootless() {
		pausePidPath, err := util.GetRootlessPauseProcessPidPathGivenDir(r.config.Engine.TmpDir)
		if err != nil {
			return errors.Wrapf(err, "could not get pause process pid file path")
		}
		data, err := ioutil.ReadFile(pausePidPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return errors.Wrap(err, "cannot read pause process pid file")
		}
		pausePid, err := strconv.Atoi(string(data))
		if err != nil {
			return errors.Wrapf(err, "cannot parse pause pid file %s", pausePidPath)
		}
		if err := os.Remove(pausePidPath); err != nil {
			return errors.Wrapf(err, "cannot delete pause pid file %s", pausePidPath)
		}
		if err := syscall.Kill(pausePid, syscall.SIGKILL); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) migrate(ctx context.Context) error {
	runningContainers, err := r.GetRunningContainers()
	if err != nil {
		return err
	}

	allCtrs, err := r.state.AllContainers()
	if err != nil {
		return err
	}

	logrus.Infof("Stopping all containers")
	for _, ctr := range runningContainers {
		fmt.Printf("stopped %s\n", ctr.ID())
		if err := ctr.Stop(); err != nil {
			return errors.Wrapf(err, "cannot stop container %s", ctr.ID())
		}
	}

	// Did the user request a new runtime?
	runtimeChangeRequested := r.migrateRuntime != ""
	requestedRuntime, runtimeExists := r.ociRuntimes[r.migrateRuntime]
	if !runtimeExists && runtimeChangeRequested {
		return errors.Wrapf(define.ErrInvalidArg, "change to runtime %q requested but no such runtime is defined", r.migrateRuntime)
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
				return errors.Wrapf(err, "error rewriting config for container %s", ctr.ID())
			}
		}
	}

	return r.stopPauseProcess()
}
