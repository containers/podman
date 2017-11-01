package main

import (
	"fmt"
	"os"

	"github.com/docker/docker/pkg/signal"
	"github.com/kubernetes-incubator/cri-o/libkpod"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	killFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "signal, s",
			Usage: "Signal to send to the container",
			Value: "KILL",
		},
	}
	killDescription = "The main process inside each container specified will be sent SIGKILL, or any signal specified with option --signal."
	killCommand     = cli.Command{
		Name:        "kill",
		Usage:       "Kill one or more running containers with a specific signal",
		Description: killDescription,
		Flags:       killFlags,
		Action:      killCmd,
		ArgsUsage:   "[CONTAINER_NAME_OR_ID]",
	}
)

// killCmd kills one or more containers with a signal
func killCmd(c *cli.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return errors.Errorf("specify one or more containers to kill")
	}
	if err := validateFlags(c, killFlags); err != nil {
		return err
	}
	config, err := getConfig(c)
	if err != nil {
		return errors.Wrapf(err, "could not get config")
	}
	server, err := libkpod.New(config)
	if err != nil {
		return errors.Wrapf(err, "could not get container server")
	}
	killSignal := c.String("signal")
	// Check if the signalString provided by the user is valid
	// Invalid signals will return err
	sysSignal, err := signal.ParseSignal(killSignal)
	if err != nil {
		return err
	}
	defer server.Shutdown()
	err = server.Update()
	if err != nil {
		return errors.Wrapf(err, "could not update list of containers")
	}
	var lastError error
	for _, container := range c.Args() {
		id, err := server.ContainerKill(container, sysSignal)
		if err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "unable to kill %v", container)
		} else {
			fmt.Println(id)
		}
	}
	return lastError
}
