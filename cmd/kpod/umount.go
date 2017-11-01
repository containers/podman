package main

import (
	"github.com/pkg/errors"
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
	args := c.Args()
	if len(args) == 0 {
		return errors.Errorf("container ID must be specified")
	}
	if len(args) > 1 {
		return errors.Errorf("too many arguments specified")
	}
	config, err := getConfig(c)
	if err != nil {
		return errors.Wrapf(err, "Could not get config")
	}
	store, err := getStore(config)
	if err != nil {
		return err
	}

	err = store.Unmount(args[0])
	if err != nil {
		return errors.Wrapf(err, "error unmounting container %q", args[0])
	}
	return nil
}
