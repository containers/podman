// +build ABISupport

package abi

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/containers/common/pkg/config"
	"github.com/containers/libpod/libpod/define"
	api "github.com/containers/libpod/pkg/api/server"
	"github.com/containers/libpod/pkg/cgroups"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/util"
	iopodman "github.com/containers/libpod/pkg/varlink"
	iopodmanAPI "github.com/containers/libpod/pkg/varlinkapi"
	"github.com/containers/libpod/utils"
	"github.com/containers/libpod/version"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/varlink/go/varlink"
)

func (ic *ContainerEngine) Info(ctx context.Context) (*define.Info, error) {
	return ic.Libpod.Info()
}

func (ic *ContainerEngine) RestService(_ context.Context, opts entities.ServiceOptions) error {
	var (
		listener *net.Listener
		err      error
	)

	if opts.URI != "" {
		fields := strings.Split(opts.URI, ":")
		if len(fields) == 1 {
			return errors.Errorf("%s is an invalid socket destination", opts.URI)
		}
		address := strings.Join(fields[1:], ":")
		l, err := net.Listen(fields[0], address)
		if err != nil {
			return errors.Wrapf(err, "unable to create socket %s", opts.URI)
		}
		listener = &l
	}

	server, err := api.NewServerWithSettings(ic.Libpod, opts.Timeout, listener)
	if err != nil {
		return err
	}
	defer func() {
		if err := server.Shutdown(); err != nil {
			logrus.Warnf("Error when stopping API service: %s", err)
		}
	}()

	err = server.Serve()
	if listener != nil {
		_ = (*listener).Close()
	}
	return err
}

func (ic *ContainerEngine) VarlinkService(_ context.Context, opts entities.ServiceOptions) error {
	var varlinkInterfaces = []*iopodman.VarlinkInterface{
		iopodmanAPI.New(opts.Command, ic.Libpod),
	}

	service, err := varlink.NewService(
		"Atomic",
		"podman",
		version.Version,
		"https://github.com/containers/libpod",
	)
	if err != nil {
		return errors.Wrapf(err, "unable to create new varlink service")
	}

	for _, i := range varlinkInterfaces {
		if err := service.RegisterInterface(i); err != nil {
			return errors.Errorf("unable to register varlink interface %v", i)
		}
	}

	// Run the varlink server at the given address
	if err = service.Listen(opts.URI, opts.Timeout); err != nil {
		switch err.(type) {
		case varlink.ServiceTimeoutError:
			logrus.Infof("varlink service expired (use --timeout to increase session time beyond %s ms, 0 means never timeout)", opts.Timeout.String())
			return nil
		default:
			return errors.Wrapf(err, "unable to start varlink service")
		}
	}
	return nil
}

func (ic *ContainerEngine) SetupRootless(_ context.Context, cmd *cobra.Command) error {
	// do it only after podman has already re-execed and running with uid==0.
	if os.Geteuid() == 0 {
		ownsCgroup, err := cgroups.UserOwnsCurrentSystemdCgroup()
		if err != nil {
			logrus.Warnf("Failed to detect the owner for the current cgroup: %v", err)
		}
		if !ownsCgroup {
			conf, err := ic.Config(context.Background())
			if err != nil {
				return err
			}
			unitName := fmt.Sprintf("podman-%d.scope", os.Getpid())
			if err := utils.RunUnderSystemdScope(os.Getpid(), "user.slice", unitName); err != nil {
				if conf.Engine.CgroupManager == config.SystemdCgroupsManager {
					logrus.Warnf("Failed to add podman to systemd sandbox cgroup: %v", err)
				} else {
					logrus.Debugf("Failed to add podman to systemd sandbox cgroup: %v", err)
				}
			}
		}
	}

	pausePidPath, err := util.GetRootlessPauseProcessPidPath()
	if err != nil {
		return errors.Wrapf(err, "could not get pause process pid file path")
	}

	became, ret, err := rootless.TryJoinPauseProcess(pausePidPath)
	if err != nil {
		return err
	}
	if became {
		os.Exit(ret)
	}

	// if there is no pid file, try to join existing containers, and create a pause process.
	ctrs, err := ic.Libpod.GetRunningContainers()
	if err != nil {
		logrus.Error(err.Error())
		os.Exit(1)
	}

	paths := []string{}
	for _, ctr := range ctrs {
		paths = append(paths, ctr.Config().ConmonPidFile)
	}

	became, ret, err = rootless.TryJoinFromFilePaths(pausePidPath, true, paths)
	if err := movePauseProcessToScope(); err != nil {
		conf, err := ic.Config(context.Background())
		if err != nil {
			return err
		}
		if conf.Engine.CgroupManager == config.SystemdCgroupsManager {
			logrus.Warnf("Failed to add pause process to systemd sandbox cgroup: %v", err)
		} else {
			logrus.Debugf("Failed to add pause process to systemd sandbox cgroup: %v", err)
		}
	}
	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
	if became {
		os.Exit(ret)
	}
	return nil
}

func movePauseProcessToScope() error {
	pausePidPath, err := util.GetRootlessPauseProcessPidPath()
	if err != nil {
		return errors.Wrapf(err, "could not get pause process pid file path")
	}

	data, err := ioutil.ReadFile(pausePidPath)
	if err != nil {
		return errors.Wrapf(err, "cannot read pause pid file")
	}
	pid, err := strconv.ParseUint(string(data), 10, 0)
	if err != nil {
		return errors.Wrapf(err, "cannot parse pid file %s", pausePidPath)
	}

	return utils.RunUnderSystemdScope(int(pid), "user.slice", "podman-pause.scope")
}

func setRLimits() error { // nolint:deadcode,unused
	rlimits := new(syscall.Rlimit)
	rlimits.Cur = 1048576
	rlimits.Max = 1048576
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, rlimits); err != nil {
		if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, rlimits); err != nil {
			return errors.Wrapf(err, "error getting rlimits")
		}
		rlimits.Cur = rlimits.Max
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, rlimits); err != nil {
			return errors.Wrapf(err, "error setting new rlimits")
		}
	}
	return nil
}

func setUMask() { // nolint:deadcode,unused
	// Be sure we can create directories with 0755 mode.
	syscall.Umask(0022)
}

// checkInput can be used to verify any of the globalopt values
func checkInput() error { // nolint:deadcode,unused
	return nil
}
