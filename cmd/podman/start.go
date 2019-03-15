package main

import (
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	startCommand     cliconfig.StartValues
	startDescription = `Starts one or more containers.  The container name or ID can be used.`

	_startCommand = &cobra.Command{
		Use:   "start [flags] CONTAINER [CONTAINER...]",
		Short: "Start one or more containers",
		Long:  startDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			startCommand.InputArgs = args
			startCommand.GlobalFlags = MainGlobalOpts
			return startCmd(&startCommand)
		},
		Example: `podman start --latest
  podman start 860a4b231279 5421ab43b45
  podman start --interactive --attach imageID`,
	}
)

func init() {
	startCommand.Command = _startCommand
	startCommand.SetHelpTemplate(HelpTemplate())
	startCommand.SetUsageTemplate(UsageTemplate())
	flags := startCommand.Flags()
	flags.BoolVarP(&startCommand.Attach, "attach", "a", false, "Attach container's STDOUT and STDERR")
	flags.StringVar(&startCommand.DetachKeys, "detach-keys", "", "Override the key sequence for detaching a container. Format is a single character [a-Z] or ctrl-<value> where <value> is one of: a-z, @, ^, [, , or _")
	flags.BoolVarP(&startCommand.Interactive, "interactive", "i", false, "Keep STDIN open even if not attached")
	flags.BoolVarP(&startCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.BoolVar(&startCommand.SigProxy, "sig-proxy", false, "Proxy received signals to the process (default true if attaching, false otherwise)")
	markFlagHiddenForRemoteClient("latest", flags)
}

func startCmd(c *cliconfig.StartValues) error {
	if c.Bool("trace") {
		span, _ := opentracing.StartSpanFromContext(Ctx, "startCmd")
		defer span.Finish()
	}

	args := c.InputArgs
	if len(args) < 1 && !c.Latest {
		return errors.Errorf("you must provide at least one container name or id")
	}

	attach := c.Attach

	if len(args) > 1 && attach {
		return errors.Errorf("you cannot start and attach multiple containers at once")
	}

	sigProxy := c.SigProxy || attach

	if sigProxy && !attach {
		return errors.Wrapf(libpod.ErrInvalidArg, "you cannot use sig-proxy without --attach")
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
			// If the container is in a pod, also set to recursively start dependencies
			err = startAttachCtr(ctr, os.Stdout, os.Stderr, inputStream, c.DetachKeys, sigProxy, !ctrRunning, ctr.PodID() != "")
			if errors.Cause(err) == libpod.ErrDetach {
				// User manually detached
				// Exit cleanly immediately
				exitCode = 0
				return nil
			}

			if ctrRunning {
				return err
			}

			if err != nil {
				return errors.Wrapf(err, "unable to start container %s", ctr.ID())
			}

			if ecode, err := ctr.Wait(); err != nil {
				if errors.Cause(err) == libpod.ErrNoSuchCtr {
					// The container may have been removed
					// Go looking for an exit file
					ctrExitCode, err := readExitFile(runtime.GetConfig().TmpDir, ctr.ID())
					if err != nil {
						logrus.Errorf("Cannot get exit code: %v", err)
						exitCode = 127
					} else {
						exitCode = ctrExitCode
					}
				}
			} else {
				exitCode = int(ecode)
			}

			return nil
		}
		if ctrRunning {
			fmt.Println(ctr.ID())
			continue
		}
		// Handle non-attach start
		// If the container is in a pod, also set to recursively start dependencies
		if err := ctr.Start(ctx, ctr.PodID() != ""); err != nil {
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
