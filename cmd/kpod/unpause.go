package main

import (
	"fmt"
	"github.com/kubernetes-incubator/cri-o/libkpod"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"os"
)

var (
	unpauseDescription = `
   kpod unpause

   Unpauses one or more running containers.  The container name or ID can be used.
`
	unpauseCommand = cli.Command{
		Name:        "unpause",
		Usage:       "Unpause the processes in one or more containers",
		Description: unpauseDescription,
		Action:      unpauseCmd,
		ArgsUsage:   "CONTAINER-NAME [CONTAINER-NAME ...]",
	}
)

func unpauseCmd(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 {
		return errors.Errorf("you must provide at least one container name or id")
	}

	config, err := getConfig(c)
	if err != nil {
		return errors.Wrapf(err, "could not get config")
	}
	server, err := libkpod.New(config)
	if err != nil {
		return errors.Wrapf(err, "could not get container server")
	}
	defer server.Shutdown()
	if err := server.Update(); err != nil {
		return errors.Wrapf(err, "could not update list of containers")
	}
	var lastError error
	for _, container := range c.Args() {
		cid, err := server.ContainerUnpause(container)
		if err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "failed to unpause container %v", container)
		} else {
			fmt.Println(cid)
		}
	}

	return lastError
}
