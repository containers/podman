package main

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	defaultTimeout int64 = 10
	stopFlags            = []cli.Flag{
		cli.Int64Flag{
			Name:  "timeout, t",
			Usage: "Seconds to wait for stop before killing the container",
			Value: defaultTimeout,
		},
	}
	stopDescription = `
   kpod stop

   Stops one or more running containers.  The container name or ID can be used.
   A timeout to forcibly stop the container can also be set but defaults to 10
   seconds otherwise.
`

	stopCommand = cli.Command{
		Name:        "stop",
		Usage:       "Stop one or more containers",
		Description: stopDescription,
		Flags:       stopFlags,
		Action:      stopCmd,
		ArgsUsage:   "CONTAINER-NAME [CONTAINER-NAME ...]",
	}
)

func stopCmd(c *cli.Context) error {
	args := c.Args()
	stopTimeout := c.Int64("timeout")
	if len(args) < 1 {
		return errors.Errorf("you must provide at least one container name or id")
	}
	if err := validateFlags(c, stopFlags); err != nil {
		return err
	}

	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	logrus.Debugf("Stopping containers with timeout %d", stopTimeout)

	var lastError error
	for _, container := range c.Args() {
		ctr, err := runtime.LookupContainer(container)
		if err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "failed to stop container %v", container)
			continue
		}

		if err := ctr.Stop(stopTimeout); err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "failed to stop container %v", container)
		} else {
			fmt.Println(ctr.ID())
		}
	}

	return lastError
}
