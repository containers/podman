package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

func getDescriptorString() string {
	descriptors, err := libpod.GetContainerPidInformationDescriptors()
	if err == nil {
		return fmt.Sprintf(`
Format Descriptors:
%s`, strings.Join(descriptors, ","))
	}
	return ""
}

var (
	topFlags = []cli.Flag{
		LatestFlag,
		cli.BoolFlag{
			Name:   "list-descriptors",
			Hidden: true,
		},
	}
	topDescription = fmt.Sprintf(`Display the running processes of the container.  Specify format descriptors
to alter the output.  You may run "podman top -l pid pcpu seccomp" to print
the process ID, the CPU percentage and the seccomp mode of each process of
the latest container.
%s
`, getDescriptorString())

	topCommand = cli.Command{
		Name:           "top",
		Usage:          "Display the running processes of a container",
		Description:    topDescription,
		Flags:          topFlags,
		Action:         topCmd,
		ArgsUsage:      "CONTAINER-NAME [format descriptors]",
		SkipArgReorder: true,
		OnUsageError:   usageErrorHandler,
	}
)

func topCmd(c *cli.Context) error {
	var container *libpod.Container
	var err error
	args := c.Args()

	if c.Bool("list-descriptors") {
		descriptors, err := libpod.GetContainerPidInformationDescriptors()
		if err != nil {
			return err
		}
		fmt.Println(strings.Join(descriptors, "\n"))
		return nil
	}

	if len(args) < 1 && !c.Bool("latest") {
		return errors.Errorf("you must provide the name or id of a running container")
	}
	if err := validateFlags(c, topFlags); err != nil {
		return err
	}

	rootless.SetSkipStorageSetup(true)
	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	var descriptors []string
	if c.Bool("latest") {
		descriptors = args
		container, err = runtime.GetLatestContainer()
	} else {
		descriptors = args[1:]
		container, err = runtime.LookupContainer(args[0])
	}

	if err != nil {
		return errors.Wrapf(err, "unable to lookup requested container")
	}

	conStat, err := container.State()
	if err != nil {
		return errors.Wrapf(err, "unable to look up state for %s", args[0])
	}
	if conStat != libpod.ContainerStateRunning {
		return errors.Errorf("top can only be used on running containers")
	}

	pid, err := container.PID()
	if err != nil {
		return err
	}
	became, ret, err := rootless.JoinNS(uint(pid))
	if err != nil {
		return err
	}
	if became {
		os.Exit(ret)
	}
	psOutput, err := container.GetContainerPidInformation(descriptors)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 5, 1, 3, ' ', 0)
	for _, proc := range psOutput {
		fmt.Fprintln(w, proc)
	}
	w.Flush()
	return nil
}
