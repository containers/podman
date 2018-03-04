package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
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

	runtime, err := getRuntime(c)
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

		if err := ctr.Init(); err != nil && errors.Cause(err) != libpod.ErrCtrExists {
			return err
		}

		// We can only be interactive if both the config and the command-line say so
		if c.Bool("interactive") && !ctr.Config().Stdin {
			return errors.Errorf("the container was not created with the interactive option")
		}
		noStdIn := c.Bool("interactive")
		tty, err := strconv.ParseBool(ctr.Spec().Annotations["io.kubernetes.cri-o.TTY"])
		if err != nil {
			return errors.Wrapf(err, "unable to parse annotations in %s", ctr.ID())
		}

		// Handle start --attach
		// We only get a terminal session if both a tty was specified in the spec and
		// -a on the command-line was given.
		if attach && tty {
			attachChan, err := ctr.StartAndAttach(noStdIn, c.String("detach-keys"))
			if err != nil {
				return errors.Wrapf(err, "unable to start container %s", ctr.ID())
			}

			if c.Bool("sig-proxy") {
				ProxySignals(ctr)
			}

			// Wait for attach to complete
			err = <-attachChan
			if err != nil {
				return errors.Wrapf(err, "error attaching to container %s", ctr.ID())
			}

			if ecode, err := ctr.ExitCode(); err != nil {
				logrus.Errorf("unable to get exit code of container %s: %q", ctr.ID(), err)
			} else {
				exitCode = int(ecode)
			}

			return ctr.Cleanup()
		}

		// Handle non-attach start
		if err := ctr.Start(); err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "unable to start container %q", container)
			continue
		}
		fmt.Println(ctr.ID())
	}

	return lastError
}
