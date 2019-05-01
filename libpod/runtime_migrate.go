package libpod

import (
	"context"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

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
		logrus.Infof("stopping %s", ctr.ID())
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

	for _, ctr := range runningContainers {
		if err := ctr.Start(ctx, true); err != nil {
			logrus.Errorf("error restarting container %s", ctr.ID())
		}
	}

	return nil
}
