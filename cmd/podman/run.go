package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	runCommand cliconfig.RunValues

	runDescription = "Runs a command in a new container from the given image"
	_runCommand    = &cobra.Command{
		Use:   "run",
		Short: "Run a command in a new container",
		Long:  runDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			runCommand.InputArgs = args
			runCommand.GlobalFlags = MainGlobalOpts
			return runCmd(&runCommand)
		},
		Example: "IMAGE [COMMAND [ARG...]]",
	}
)

func init() {
	runCommand.Command = _runCommand
	runCommand.SetUsageTemplate(UsageTemplate())
	flags := runCommand.Flags()
	flags.SetInterspersed(false)
	flags.Bool("sig-proxy", true, "Proxy received signals to the process (default true)")
	getCreateFlags(&runCommand.PodmanCommand)
}

func runCmd(c *cliconfig.RunValues) error {
	if err := createInit(&c.PodmanCommand); err != nil {
		return err
	}
	if os.Geteuid() != 0 {
		rootless.SetSkipStorageSetup(true)
	}

	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	ctr, createConfig, err := createContainer(&c.PodmanCommand, runtime)
	if err != nil {
		return err
	}

	if logrus.GetLevel() == logrus.DebugLevel {
		cgroupPath, err := ctr.CGroupPath()
		if err == nil {
			logrus.Debugf("container %q has CgroupParent %q", ctr.ID(), cgroupPath)
		}
	}

	ctx := getContext()
	// Handle detached start
	if createConfig.Detach {
		if err := ctr.Start(ctx); err != nil {
			// This means the command did not exist
			exitCode = 127
			if strings.Index(err.Error(), "permission denied") > -1 {
				exitCode = 126
			}
			return err
		}

		fmt.Printf("%s\n", ctr.ID())
		exitCode = 0
		return nil
	}

	outputStream := os.Stdout
	errorStream := os.Stderr
	inputStream := os.Stdin

	// If -i is not set, clear stdin
	if !c.Bool("interactive") {
		inputStream = nil
	}

	// If attach is set, clear stdin/stdout/stderr and only attach requested
	if c.IsSet("attach") || c.IsSet("a") {
		outputStream = nil
		errorStream = nil
		if !c.Bool("interactive") {
			inputStream = nil
		}

		attachTo := c.StringSlice("attach")
		for _, stream := range attachTo {
			switch strings.ToLower(stream) {
			case "stdout":
				outputStream = os.Stdout
			case "stderr":
				errorStream = os.Stderr
			case "stdin":
				inputStream = os.Stdin
			default:
				return errors.Wrapf(libpod.ErrInvalidArg, "invalid stream %q for --attach - must be one of stdin, stdout, or stderr", stream)
			}
		}
	}
	if err := startAttachCtr(ctr, outputStream, errorStream, inputStream, c.String("detach-keys"), c.Bool("sig-proxy"), true); err != nil {
		// We've manually detached from the container
		// Do not perform cleanup, or wait for container exit code
		// Just exit immediately
		if err == libpod.ErrDetach {
			exitCode = 0
			return nil
		}

		// This means the command did not exist
		exitCode = 127
		if strings.Index(err.Error(), "permission denied") > -1 {
			exitCode = 126
		}
		if c.IsSet("rm") {
			if deleteError := runtime.RemoveContainer(ctx, ctr, true); deleteError != nil {
				logrus.Errorf("unable to remove container %s after failing to start and attach to it", ctr.ID())
			}
		}
		return err
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

// Read a container's exit file
func readExitFile(runtimeTmp, ctrID string) (int, error) {
	exitFile := filepath.Join(runtimeTmp, "exits", fmt.Sprintf("%s-old", ctrID))

	logrus.Debugf("Attempting to read container %s exit code from file %s", ctrID, exitFile)

	// Check if it exists
	if _, err := os.Stat(exitFile); err != nil {
		return 0, errors.Wrapf(err, "error getting exit file for container %s", ctrID)
	}

	// File exists, read it in and convert to int
	statusStr, err := ioutil.ReadFile(exitFile)
	if err != nil {
		return 0, errors.Wrapf(err, "error reading exit file for container %s", ctrID)
	}

	exitCode, err := strconv.Atoi(string(statusStr))
	if err != nil {
		return 0, errors.Wrapf(err, "error parsing exit code for container %s", ctrID)
	}

	return exitCode, nil
}
