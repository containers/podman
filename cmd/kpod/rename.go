package main

import (
	"github.com/kubernetes-incubator/cri-o/libkpod"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	renameDescription = "Rename a container.  Container may be created, running, paused, or stopped"
	renameFlags       = []cli.Flag{}
	renameCommand     = cli.Command{
		Name:        "rename",
		Usage:       "rename a container",
		Description: renameDescription,
		Action:      renameCmd,
		ArgsUsage:   "CONTAINER NEW-NAME",
		Flags:       renameFlags,
	}
)

func renameCmd(c *cli.Context) error {
	if len(c.Args()) != 2 {
		return errors.Errorf("Rename requires a src container name/ID and a dest container name")
	}
	if err := validateFlags(c, renameFlags); err != nil {
		return err
	}

	config, err := getConfig(c)
	if err != nil {
		return errors.Wrapf(err, "Could not get config")
	}
	server, err := libkpod.New(config)
	if err != nil {
		return errors.Wrapf(err, "could not get container server")
	}
	defer server.Shutdown()
	err = server.Update()
	if err != nil {
		return errors.Wrapf(err, "could not update list of containers")
	}

	err = server.ContainerRename(c.Args().Get(0), c.Args().Get(1))
	if err != nil {
		return errors.Wrapf(err, "could not rename container")
	}
	return nil
}
