package main

import (
	"fmt"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"strconv"

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
		Use:   "exec [flags] CONTAINER [COMMAND [ARG...]]",
		Short: "Run a process in a running container",
		Long:  execDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			execCommand.InputArgs = args
			execCommand.GlobalFlags = MainGlobalOpts
			return execCmd(&execCommand)
		},
		Example: `podman exec -it ctrID ls
  podman exec -it -w /tmp myCtr pwd
  podman exec --user root ctrID ls`,
	}
)

func init() {
	execCommand.Command = _execCommand
	execCommand.SetUsageTemplate(UsageTemplate())
	flags := execCommand.Flags()
	flags.SetInterspersed(false)
	flags.StringSliceVarP(&execCommand.Env, "env", "e", []string{}, "Set environment variables")
	flags.BoolVarP(&execCommand.Interfactive, "interactive", "i", false, "Not supported.  All exec commands are interactive by default")
	flags.BoolVarP(&execCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.BoolVar(&execCommand.Privileged, "privileged", false, "Give the process extended Linux capabilities inside the container.  The default is false")
	flags.BoolVarP(&execCommand.Tty, "tty", "t", false, "Allocate a pseudo-TTY. The default is false")
	flags.StringVarP(&execCommand.User, "user", "u", "", "Sets the username or UID used and optionally the groupname or GID for the specified command")

	flags.IntVar(&execCommand.PreserveFDs, "preserve-fds", 0, "Pass N additional file descriptors to the container")
	flags.StringVarP(&execCommand.Workdir, "workdir", "w", "", "Working directory inside the container")
	markFlagHiddenForRemoteClient("latest", flags)
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

	if c.PreserveFDs > 0 {
		entries, err := ioutil.ReadDir("/proc/self/fd")
		if err != nil {
			return errors.Wrapf(err, "unable to read /proc/self/fd")
		}
		m := make(map[int]bool)
		for _, e := range entries {
			i, err := strconv.Atoi(e.Name())
			if err != nil {
				if err != nil {
					return errors.Wrapf(err, "cannot parse %s in /proc/self/fd", e.Name())
				}
			}
			m[i] = true
		}
		for i := 3; i < 3+c.PreserveFDs; i++ {
			if _, found := m[i]; !found {
				return errors.New("invalid --preserve-fds=N specified. Not enough FDs available")
			}
		}

	}

	if os.Geteuid() != 0 {
		var became bool
		var ret int

		data, err := ioutil.ReadFile(ctr.Config().ConmonPidFile)
		if err != nil {
			return errors.Wrapf(err, "cannot read conmon PID file %q", ctr.Config().ConmonPidFile)
		}
		conmonPid, err := strconv.Atoi(string(data))
		if err != nil {
			return errors.Wrapf(err, "cannot parse PID %q", data)
		}
		became, ret, err = rootless.JoinDirectUserAndMountNS(uint(conmonPid))
		if err != nil {
			return err
		}
		if became {
			os.Exit(ret)
		}
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

	streams := new(libpod.AttachStreams)
	streams.OutputStream = os.Stdout
	streams.ErrorStream = os.Stderr
	streams.InputStream = os.Stdin
	streams.AttachOutput = true
	streams.AttachError = true
	streams.AttachInput = true

	return ctr.Exec(c.Tty, c.Privileged, envs, cmd, c.User, c.Workdir, streams, c.PreserveFDs)
}
