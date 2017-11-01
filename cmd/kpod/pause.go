package main

import (
	"fmt"
	"github.com/kubernetes-incubator/cri-o/libkpod"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"os"
)

var (
	pauseDescription = `
   kpod pause

   Pauses one or more running containers.  The container name or ID can be used.
`
	pauseCommand = cli.Command{
		Name:        "pause",
		Usage:       "Pauses all the processes in one or more containers",
		Description: pauseDescription,
		Action:      pauseCmd,
		ArgsUsage:   "CONTAINER-NAME [CONTAINER-NAME ...]",
	}
)

func pauseCmd(c *cli.Context) error {
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
		cid, err := server.ContainerPause(container)
		if err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "failed to pause container %v", container)
		} else {
			fmt.Println(cid)
		}
	}

	return lastError
}
