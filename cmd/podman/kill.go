package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/docker/docker/pkg/signal"
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

	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	var killSignal uint = uint(syscall.SIGTERM)
	if c.String("signal") != "" {
		// Check if the signalString provided by the user is valid
		// Invalid signals will return err
		sysSignal, err := signal.ParseSignal(c.String("signal"))
		if err != nil {
			return err
		}
		killSignal = uint(sysSignal)
	}

	var lastError error
	for _, container := range c.Args() {
		ctr, err := runtime.LookupContainer(container)
		if err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "unable to find container %v", container)
			continue
		}

		if err := ctr.Kill(killSignal); err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "unable to find container %v", container)
		} else {
			fmt.Println(ctr.ID())
		}
	}
	return lastError
}
