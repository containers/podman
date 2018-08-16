package main

import (
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	startFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "attach, a",
			Usage: "Attach container's STDOUT and STDERR",
		},
		cli.StringFlag{
			Name:  "detach-keys",
			Usage: "Override the key sequence for detaching a container. Format is a single character [a-Z] or ctrl-<value> where <value> is one of: a-z, @, ^, [, , or _.",
		},
		cli.BoolFlag{
			Name:  "interactive, i",
			Usage: "Keep STDIN open even if not attached",
		},
		cli.BoolFlag{
			Name:  "sig-proxy",
			Usage: "proxy received signals to the process",
		},
		LatestFlag,
	}
	startDescription = `
   podman start

   Starts one or more containers.  The container name or ID can be used.
`

	startCommand = cli.Command{
		Name:                   "start",
		Usage:                  "Start one or more containers",
		Description:            startDescription,
		Flags:                  startFlags,
		Action:                 startCmd,
		ArgsUsage:              "CONTAINER-NAME [CONTAINER-NAME ...]",
		UseShortOptionHandling: true,
	}
)

func startCmd(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 && !c.Bool("latest") {
		return errors.Errorf("you must provide at least one container name or id")
	}

	attach := c.Bool("attach")

	if len(args) > 1 && attach {
		return errors.Errorf("you cannot start and attach multiple containers at once")
	}

	if err := validateFlags(c, startFlags); err != nil {
		return err
	}

	if c.Bool("sig-proxy") && !attach {
		return errors.Wrapf(libpod.ErrInvalidArg, "you cannot use sig-proxy without --attach")
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)
	if c.Bool("latest") {
		lastCtr, err := runtime.GetLatestContainer()
		if err != nil {
			return errors.Wrapf(err, "unable to get latest container")
		}
		args = append(args, lastCtr.ID())
	}
	var lastError error
	for _, container := range args {
		ctr, err := runtime.LookupContainer(container)
		if err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "unable to find container %s", container)
			continue
		}

		ctrState, err := ctr.State()
		if err != nil {
			return errors.Wrapf(err, "unable to get container state")
		}

		ctrRunning := ctrState == libpod.ContainerStateRunning

		if attach {
			inputStream := os.Stdin
			if !c.Bool("interactive") {
				inputStream = nil
			}

			// attach to the container and also start it not already running
			err = startAttachCtr(ctr, os.Stdout, os.Stderr, inputStream, c.String("detach-keys"), c.Bool("sig-proxy"), !ctrRunning)
			if ctrRunning {
				return err
			}

			if err != nil {
				return errors.Wrapf(err, "unable to start container %s", ctr.ID())
			}

			if ecode, err := ctr.Wait(); err != nil {
				logrus.Errorf("unable to get exit code of container %s: %q", ctr.ID(), err)
			} else {
				exitCode = int(ecode)
			}

			return ctr.Cleanup()
		}
		if ctrRunning {
			fmt.Println(ctr.ID())
			continue
		}
		// Handle non-attach start
		if err := ctr.Start(getContext()); err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "unable to start container %q", container)
			continue
		}
		fmt.Println(container)
	}

	return lastError
}
