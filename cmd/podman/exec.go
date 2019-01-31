package main

import (
	"fmt"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/spf13/cobra"
	"os"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
)

var (
	execCommand cliconfig.ExecValues

	execDescription = `
	podman exec

	Run a command in a running container
`
	_execCommand = &cobra.Command{
		Use:   "exec",
		Short: "Run a process in a running container",
		Long:  execDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			execCommand.InputArgs = args
			execCommand.GlobalFlags = MainGlobalOpts
			return execCmd(&execCommand)
		},
		Example: "CONTAINER-NAME",
	}
)

func init() {
	execCommand.Command = _execCommand
	flags := execCommand.Flags()
	flags.SetInterspersed(false)
	flags.StringSliceVarP(&execCommand.Env, "env", "e", []string{}, "Set environment variables")
	flags.BoolVarP(&execCommand.Interfactive, "interactive", "i", false, "Not supported.  All exec commands are interactive by default")
	flags.BoolVarP(&execCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.BoolVar(&execCommand.Privileged, "privileged", false, "Give the process extended Linux capabilities inside the container.  The default is false")
	flags.BoolVarP(&execCommand.Tty, "tty", "t", false, "Allocate a pseudo-TTY. The default is false")
	flags.StringVarP(&execCommand.User, "user", "u", "", "Sets the username or UID used and optionally the groupname or GID for the specified command")

	flags.StringVarP(&execCommand.Workdir, "workdir", "w", "", "Working directory inside the container")

	rootCmd.AddCommand(execCommand.Command)
}

func execCmd(c *cliconfig.ExecValues) error {
	args := c.InputArgs
	var ctr *libpod.Container
	var err error
	argStart := 1
	if len(args) < 1 && !c.Latest {
		return errors.Errorf("you must provide one container name or id")
	}
	if len(args) < 2 && !c.Latest {
		return errors.Errorf("you must provide a command to exec")
	}
	if c.Latest {
		argStart = 0
	}
	rootless.SetSkipStorageSetup(true)
	cmd := args[argStart:]
	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	if c.Latest {
		ctr, err = runtime.GetLatestContainer()
	} else {
		ctr, err = runtime.LookupContainer(args[0])
	}
	if err != nil {
		return errors.Wrapf(err, "unable to exec into %s", args[0])
	}

	pid, err := ctr.PID()
	if err != nil {
		return err
	}
	became, ret, err := rootless.JoinNS(uint(pid))
	if err != nil {
		return err
	}
	if became {
		os.Exit(ret)
	}

	// ENVIRONMENT VARIABLES
	env := map[string]string{}

	if err := readKVStrings(env, []string{}, c.Env); err != nil {
		return errors.Wrapf(err, "unable to process environment variables")
	}
	envs := []string{}
	for k, v := range env {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}

	return ctr.Exec(c.Tty, c.Privileged, envs, cmd, c.User, c.Workdir)
}
