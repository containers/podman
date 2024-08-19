//go:build !remote

package abi

import (
	"context"
	"fmt"
	"os"

	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/systemd"
	"github.com/containers/podman/v5/pkg/rootless"
	"github.com/containers/podman/v5/pkg/util"
	"github.com/containers/storage/pkg/unshare"
	"github.com/sirupsen/logrus"
)

// Default path for system runtime state
const defaultRunPath = "/run"

func (ic *ContainerEngine) SetupRootless(_ context.Context, noMoveProcess bool, cgroupMode string) error {
	runsUnderSystemd := systemd.RunsOnSystemd()
	if !runsUnderSystemd {
		isPid1 := os.Getpid() == 1
		if _, found := os.LookupEnv("container"); isPid1 || found {
			if err := cgroups.MaybeMoveToSubCgroup(); err != nil {
				// it is a best effort operation, so just print the
				// error for debugging purposes.
				logrus.Debugf("Could not move to subcgroup: %v", err)
			}
		}
	}

	hasCapSysAdmin, err := unshare.HasCapSysAdmin()
	if err != nil {
		return err
	}

	// check for both euid == 0 and CAP_SYS_ADMIN because we may be running in a container with CAP_SYS_ADMIN set.
	if os.Geteuid() == 0 && hasCapSysAdmin {
		// do it only after podman has already re-execed and running with uid==0.
		configureCgroup := cgroupMode != "disabled"
		if configureCgroup {
			ownsCgroup, err := cgroups.UserOwnsCurrentSystemdCgroup()
			if err != nil {
				logrus.Infof("Failed to detect the owner for the current cgroup: %v", err)
			}
			if !ownsCgroup {
				conf, err := ic.Config(context.Background())
				if err != nil {
					return err
				}
				unitName := fmt.Sprintf("podman-%d.scope", os.Getpid())
				if runsUnderSystemd || conf.Engine.CgroupManager == config.SystemdCgroupsManager {
					if err := systemd.RunUnderSystemdScope(os.Getpid(), "user.slice", unitName); err != nil {
						logrus.Debugf("Failed to add podman to systemd sandbox cgroup: %v", err)
					}
				}
			}
		}
		return nil
	}

	pausePidPath, err := util.GetRootlessPauseProcessPidPath()
	if err != nil {
		return fmt.Errorf("could not get pause process pid file path: %w", err)
	}

	became, ret, err := rootless.TryJoinPauseProcess(pausePidPath)
	if err != nil {
		return err
	}
	if became {
		os.Exit(ret)
	}
	if noMoveProcess {
		return nil
	}

	// if there is no pid file, try to join existing containers, and create a pause process.
	ctrs, err := ic.Libpod.GetRunningContainers()
	if err != nil {
		logrus.Error(err.Error())
		os.Exit(1)
	}

	paths := []string{}
	for _, ctr := range ctrs {
		paths = append(paths, ctr.ConfigNoCopy().ConmonPidFile)
	}

	if len(paths) > 0 {
		became, ret, err = rootless.TryJoinFromFilePaths(pausePidPath, paths)
	} else {
		became, ret, err = rootless.BecomeRootInUserNS(pausePidPath)
		if err == nil {
			systemd.MovePauseProcessToScope(pausePidPath)
		}
	}
	if err != nil {
		logrus.Error(fmt.Errorf("invalid internal status, try resetting the pause process with %q: %w", os.Args[0]+" system migrate", err))
		os.Exit(1)
	}
	if became {
		os.Exit(ret)
	}
	return nil
}
