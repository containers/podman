package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/cmd/podman/libpodruntime"
	"github.com/projectatomic/libpod/libpod"
	"github.com/urfave/cli"
)

var (
	topFlags = []cli.Flag{
		LatestFlag,
	}
	topDescription = `
   podman top

	Display the running processes of the container.
`

	topCommand = cli.Command{
		Name:           "top",
		Usage:          "Display the running processes of a container",
		Description:    topDescription,
		Flags:          topFlags,
		Action:         topCmd,
		ArgsUsage:      "CONTAINER-NAME",
		SkipArgReorder: true,
	}
)

func topCmd(c *cli.Context) error {
	var container *libpod.Container
	var err error
	args := c.Args()

	if len(args) < 1 && !c.Bool("latest") {
		return errors.Errorf("you must provide the name or id of a running container")
	}
	if err := validateFlags(c, topFlags); err != nil {
		return err
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	if c.Bool("latest") {
		container, err = runtime.GetLatestContainer()
	} else {
		container, err = runtime.LookupContainer(args[0])
	}

	if err != nil {
		return errors.Wrapf(err, "unable to lookup %s", args[0])
	}
	conStat, err := container.State()
	if err != nil {
		return errors.Wrapf(err, "unable to look up state for %s", args[0])
	}
	if conStat != libpod.ContainerStateRunning {
		return errors.Errorf("top can only be used on running containers")
	}

	psOutput, err := container.GetContainerPidInformation([]string{})
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
	for _, proc := range psOutput {
		fmt.Fprintln(w, proc)
	}
	w.Flush()
	return nil
}
