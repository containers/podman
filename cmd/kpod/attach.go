package main

import (
	"sync"

	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	attachFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "detach-keys",
			Usage: "Override the key sequence for detaching a container. Format is a single character [a-Z] or ctrl-<value> where <value> is one of: a-z, @, ^, [, , or _.",
		},
		cli.BoolFlag{
			Name:  "no-stdin",
			Usage: "Do not attach STDIN. The default is false.",
		},
	}
	attachDescription = "The kpod attach command allows you to attach to a running container using the container's ID or name, either to view its ongoing output or to control it interactively."
	attachCommand     = cli.Command{
		Name:        "attach",
		Usage:       "Attach to a running container",
		Description: attachDescription,
		Flags:       attachFlags,
		Action:      attachCmd,
		ArgsUsage:   "",
	}
)

func attachCmd(c *cli.Context) error {
	args := c.Args()
	if err := validateFlags(c, attachFlags); err != nil {
		return err
	}

	if len(c.Args()) < 1 || len(c.Args()) > 1 {
		return errors.Errorf("attach requires the name or id of one running container")
	}

	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	ctr, err := runtime.LookupContainer(args[0])

	if err != nil {
		return errors.Wrapf(err, "unable to exec into %s", args[0])
	}

	conState, err := ctr.State()
	if err != nil {
		return errors.Wrapf(err, "unable to determine state of %s", args[0])
	}
	if conState != libpod.ContainerStateRunning {
		return errors.Errorf("you can only attach to running containers")
	}
	// Create a bool channel to track that the console socket attach
	// is successful.
	attached := make(chan bool)
	// Create a waitgroup so we can sync and wait for all goroutines
	// to finish before exiting main
	var wg sync.WaitGroup

	// We increment the wg counter because we need to do the attach
	wg.Add(1)
	// Attach to the running container
	go func() {
		logrus.Debug("trying to attach to the container %s", ctr.ID())
		defer wg.Done()
		if err := ctr.Attach(c.Bool("no-stdin"), c.String("detach-keys"), attached); err != nil {
			logrus.Errorf("unable to attach to container %s: %q", ctr.ID(), err)
		}
	}()
	if !<-attached {
		return errors.Errorf("unable to attach to container %s", ctr.ID())
	}
	wg.Wait()

	return nil
}
