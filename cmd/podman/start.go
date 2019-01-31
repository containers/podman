package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	cc "github.com/containers/libpod/pkg/spec"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	startCommand     cliconfig.StartValues
	startDescription = `
   podman start

   Starts one or more containers.  The container name or ID can be used.
`
	_startCommand = &cobra.Command{
		Use:   "start",
		Short: "Start one or more containers",
		Long:  startDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			startCommand.InputArgs = args
			startCommand.GlobalFlags = MainGlobalOpts
			return startCmd(&startCommand)
		},
		Example: "CONTAINER-NAME [CONTAINER-NAME ...]",
	}
)

func init() {
	startCommand.Command = _startCommand
	flags := startCommand.Flags()
	flags.BoolVarP(&startCommand.Attach, "attach", "a", false, "Attach container's STDOUT and STDERR")
	flags.StringVar(&startCommand.DetachKeys, "detach-keys", "", "Override the key sequence for detaching a container. Format is a single character [a-Z] or ctrl-<value> where <value> is one of: a-z, @, ^, [, , or _")
	flags.BoolVarP(&startCommand.Interactive, "interactive", "i", false, "Keep STDIN open even if not attached")
	flags.BoolVarP(&startCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.BoolVar(&startCommand.SigProxy, "sig-proxy", true, "Proxy received signals to the process (default true if attaching, false otherwise)")

	rootCmd.AddCommand(startCommand.Command)
}

func startCmd(c *cliconfig.StartValues) error {
	args := c.InputArgs
	if len(args) < 1 && !c.Latest {
		return errors.Errorf("you must provide at least one container name or id")
	}

	attach := c.Attach

	if len(args) > 1 && attach {
		return errors.Errorf("you cannot start and attach multiple containers at once")
	}

	sigProxy := c.SigProxy

	if sigProxy && !attach {
		if c.Flag("sig-proxy").Changed {
			return errors.Wrapf(libpod.ErrInvalidArg, "you cannot use sig-proxy without --attach")
		} else {
			sigProxy = false
		}
	}

	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)
	if c.Latest {
		lastCtr, err := runtime.GetLatestContainer()
		if err != nil {
			return errors.Wrapf(err, "unable to get latest container")
		}
		args = append(args, lastCtr.ID())
	}

	ctx := getContext()

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
			if !c.Interactive {
				inputStream = nil
			}

			// attach to the container and also start it not already running
			err = startAttachCtr(ctr, os.Stdout, os.Stderr, inputStream, c.DetachKeys, sigProxy, !ctrRunning)
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

			return ctr.Cleanup(ctx)
		}
		if ctrRunning {
			fmt.Println(ctr.ID())
			continue
		}
		// Handle non-attach start
		if err := ctr.Start(ctx); err != nil {
			var createArtifact cc.CreateConfig
			artifact, artifactErr := ctr.GetArtifact("create-config")
			if artifactErr == nil {
				if jsonErr := json.Unmarshal(artifact, &createArtifact); jsonErr != nil {
					logrus.Errorf("unable to detect if container %s should be deleted", ctr.ID())
				}
				if createArtifact.Rm {
					if rmErr := runtime.RemoveContainer(ctx, ctr, true); rmErr != nil {
						logrus.Errorf("unable to remove container %s after it failed to start", ctr.ID())
					}
				}
			}
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
