package main

import (
	"fmt"
	"os"

	"github.com/kubernetes-incubator/cri-o/libkpod"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	waitDescription = `
	kpod wait

	Block until one or more containers stop and then print their exit codes
`

	waitCommand = cli.Command{
		Name:        "wait",
		Usage:       "Block on one or more containers",
		Description: waitDescription,
		Action:      waitCmd,
		ArgsUsage:   "CONTAINER-NAME [CONTAINER-NAME ...]",
	}
)

func waitCmd(c *cli.Context) error {
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
	err = server.Update()
	if err != nil {
		return errors.Wrapf(err, "could not update list of containers")
	}

	var lastError error
	for _, container := range c.Args() {
		returnCode, err := server.ContainerWait(container)
		if err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "failed to wait for the container %v", container)
		} else {
			fmt.Println(returnCode)
		}
	}

	return lastError
}
