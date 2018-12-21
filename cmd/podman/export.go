package main

import (
	"io/ioutil"
	"os"
	"strconv"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	exportFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "output, o",
			Usage: "Write to a file, default is STDOUT",
			Value: "/dev/stdout",
		},
	}
	exportDescription = "Exports container's filesystem contents as a tar archive" +
		" and saves it on the local machine."
	exportCommand = cli.Command{
		Name:         "export",
		Usage:        "Export container's filesystem contents as a tar archive",
		Description:  exportDescription,
		Flags:        sortFlags(exportFlags),
		Action:       exportCmd,
		ArgsUsage:    "CONTAINER",
		OnUsageError: usageErrorHandler,
	}
)

// exportCmd saves a container to a tarball on disk
func exportCmd(c *cli.Context) error {
	if err := validateFlags(c, exportFlags); err != nil {
		return err
	}
	if os.Geteuid() != 0 {
		rootless.SetSkipStorageSetup(true)
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	args := c.Args()
	if len(args) == 0 {
		return errors.Errorf("container id must be specified")
	}
	if len(args) > 1 {
		return errors.Errorf("too many arguments given, need 1 at most.")
	}

	output := c.String("output")
	if output == "/dev/stdout" {
		file := os.Stdout
		if logrus.IsTerminal(file) {
			return errors.Errorf("refusing to export to terminal. Use -o flag or redirect")
		}
	}
	if err := validateFileName(output); err != nil {
		return err
	}

	ctr, err := runtime.LookupContainer(args[0])
	if err != nil {
		return errors.Wrapf(err, "error looking up container %q", args[0])
	}

	if os.Geteuid() != 0 {
		state, err := ctr.State()
		if err != nil {
			return errors.Wrapf(err, "cannot read container state %q", ctr.ID())
		}
		if state == libpod.ContainerStateRunning || state == libpod.ContainerStatePaused {
			data, err := ioutil.ReadFile(ctr.Config().ConmonPidFile)
			if err != nil {
				return errors.Wrapf(err, "cannot read conmon PID file %q", ctr.Config().ConmonPidFile)
			}
			conmonPid, err := strconv.Atoi(string(data))
			if err != nil {
				return errors.Wrapf(err, "cannot parse PID %q", data)
			}
			became, ret, err := rootless.JoinDirectUserAndMountNS(uint(conmonPid))
			if err != nil {
				return err
			}
			if became {
				os.Exit(ret)
			}
		} else {
			became, ret, err := rootless.BecomeRootInUserNS()
			if err != nil {
				return err
			}
			if became {
				os.Exit(ret)
			}
		}
	}

	return ctr.Export(output)
}
