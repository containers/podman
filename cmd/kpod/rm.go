package main

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	rmFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "force, f",
			Usage: "Force removal of a running container.  The default is false",
		},
	}
	rmDescription = "Remove one or more containers"
	rmCommand     = cli.Command{
		Name: "rm",
		Usage: fmt.Sprintf(`kpod rm will remove one or more containers from the host.  The container name or ID can be used.
							This does not remove images.  Running containers will not be removed without the -f option.`),
		Description: rmDescription,
		Flags:       rmFlags,
		Action:      rmCmd,
		ArgsUsage:   "",
	}
)

// saveCmd saves the image to either docker-archive or oci
func rmCmd(c *cli.Context) error {
	if err := validateFlags(c, rmFlags); err != nil {
		return err
	}

	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	args := c.Args()
	if len(args) == 0 {
		return errors.Errorf("specify one or more containers to remove")
	}

	for _, container := range args {
		ctr, err := runtime.LookupContainer(container)
		if err != nil {
			return errors.Wrapf(err, "error looking up container", container)
		}
		err = runtime.RemoveContainer(ctr, c.Bool("force"))
		if err != nil {
			return errors.Wrapf(err, "error removing container %q", ctr.ID())
		}
		fmt.Println(ctr.ID())
	}
	return nil
}
