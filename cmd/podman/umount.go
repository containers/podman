package main

import (
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/cmd/podman/libpodruntime"
	"github.com/urfave/cli"
)

var (
	umountCommand = cli.Command{
		Name:        "umount",
		Aliases:     []string{"unmount"},
		Usage:       "Unmount a working container's root filesystem",
		Description: "Unmounts a working container's root filesystem",
		Action:      umountCmd,
		ArgsUsage:   "CONTAINER-NAME-OR-ID",
	}
)

func umountCmd(c *cli.Context) error {
	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	args := c.Args()
	if len(args) == 0 {
		return errors.Errorf("container ID must be specified")
	}
	if len(args) > 1 {
		return errors.Errorf("too many arguments specified")
	}

	ctr, err := runtime.LookupContainer(args[0])
	if err != nil {
		return errors.Wrapf(err, "error looking up container %q", args[0])
	}

	return ctr.Unmount()
}
