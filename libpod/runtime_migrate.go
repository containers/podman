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

	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func stopPauseProcess() error {
	if rootless.IsRootless() {
		pausePidPath, err := util.GetRootlessPauseProcessPidPath()
		if err != nil {
			return errors.Wrapf(err, "could not get pause process pid file path")
		}
		data, err := ioutil.ReadFile(pausePidPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return errors.Wrapf(err, "cannot read pause process pid file %s", pausePidPath)
		}
		pausePid, err := strconv.Atoi(string(data))
		if err != nil {
			return errors.Wrapf(err, "cannot parse pause pid file %s", pausePidPath)
		}
		if err := os.Remove(pausePidPath); err != nil {
			return errors.Wrapf(err, "cannot delete pause pid file %s", pausePidPath)
		}
		syscall.Kill(pausePid, syscall.SIGKILL)
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

	logrus.Infof("stopping all containers")
	for _, ctr := range runningContainers {
		fmt.Printf("stopped %s\n", ctr.ID())
		if err := ctr.Stop(); err != nil {
			return errors.Wrapf(err, "cannot stop container %s", ctr.ID())
		}
	}

	for _, ctr := range allCtrs {
		oldLocation := filepath.Join(ctr.state.RunDir, "conmon.pid")
		if ctr.config.ConmonPidFile == oldLocation {
			logrus.Infof("changing conmon PID file for %s", ctr.ID())
			ctr.config.ConmonPidFile = filepath.Join(ctr.config.StaticDir, "conmon.pid")
			if err := r.state.RewriteContainerConfig(ctr, ctr.config); err != nil {
				return errors.Wrapf(err, "error rewriting config for container %s", ctr.ID())
			}
		}
	}

	return stopPauseProcess()
}
