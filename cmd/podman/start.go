package main

import (
	"fmt"
	"os"
	"strconv"
	"sync"

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
		// Create a bool channel to track that the console socket attach
		// is successful.
		attached := make(chan bool)
		// Create a waitgroup so we can sync and wait for all goroutines
		// to finish before exiting main
		var wg sync.WaitGroup

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
		// We only get a terminal session if both a tty was specified in the spec and
		// -a on the command-line was given.
		if attach && tty {
			// We increment the wg counter because we need to do the attach
			wg.Add(1)
			// Attach to the running container
			go func() {
				logrus.Debugf("trying to attach to the container %s", ctr.ID())
				defer wg.Done()
				if err := ctr.Attach(noStdIn, c.String("detach-keys"), attached); err != nil {
					logrus.Errorf("unable to attach to container %s: %q", ctr.ID(), err)
				}
			}()
			if !<-attached {
				return errors.Errorf("unable to attach to container %s", ctr.ID())
			}
		}
		err = ctr.Start()
		if err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "unable to start %s", container)
			continue
		}
		if !attach {
			fmt.Println(ctr.ID())
		}
		wg.Wait()
		if ecode, err := ctr.ExitCode(); err != nil {
			logrus.Errorf("unable to get exit code of container %s: %q", ctr.ID(), err)
		} else {
			exitCode = int(ecode)
		}
	}
	return lastError
}
