package containers

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/domain/entities"
	envLib "github.com/containers/podman/v5/pkg/env"
	"github.com/containers/podman/v5/pkg/rootless"
	"github.com/spf13/cobra"
)

var (
	execDescription = `Execute the specified command inside a running container.
`
	execCommand = &cobra.Command{
		Use:               "exec [options] CONTAINER COMMAND [ARG...]",
		Short:             "Run a process in a running container",
		Long:              execDescription,
		RunE:              exec,
		ValidArgsFunction: common.AutocompleteExecCommand,
		Example: `podman exec -it ctrID ls
  podman exec -it -w /tmp myCtr pwd
  podman exec --user root ctrID ls`,
	}

	containerExecCommand = &cobra.Command{
		Use:               execCommand.Use,
		Short:             execCommand.Short,
		Long:              execCommand.Long,
		RunE:              execCommand.RunE,
		ValidArgsFunction: execCommand.ValidArgsFunction,
		Example: `podman container exec -it ctrID ls
  podman container exec -it -w /tmp myCtr pwd
  podman container exec --user root ctrID ls`,
	}
)

var (
	envInput, envFile []string
	execOpts          entities.ExecOptions
	execDetach        bool
)

func execFlags(cmd *cobra.Command) {
	podmanConfig := registry.PodmanConfig()
	flags := cmd.Flags()

	flags.SetInterspersed(false)
	flags.BoolVarP(&execDetach, "detach", "d", false, "Run the exec session in detached mode (backgrounded)")

	detachKeysFlagName := "detach-keys"
	flags.StringVar(&execOpts.DetachKeys, detachKeysFlagName, containerConfig.DetachKeys(), "Select the key sequence for detaching a container. Format is a single character [a-Z] or ctrl-<value> where <value> is one of: a-z, @, ^, [, , or _")
	_ = cmd.RegisterFlagCompletionFunc(detachKeysFlagName, common.AutocompleteDetachKeys)

	envFlagName := "env"
	flags.StringArrayVarP(&envInput, envFlagName, "e", []string{}, "Set environment variables")
	_ = cmd.RegisterFlagCompletionFunc(envFlagName, completion.AutocompleteNone)

	envFileFlagName := "env-file"
	flags.StringArrayVar(&envFile, envFileFlagName, []string{}, "Read in a file of environment variables")
	_ = cmd.RegisterFlagCompletionFunc(envFileFlagName, completion.AutocompleteDefault)

	flags.BoolVarP(&execOpts.Interactive, "interactive", "i", false, "Make STDIN available to the contained process")
	flags.BoolVar(&execOpts.Privileged, "privileged", podmanConfig.ContainersConfDefaultsRO.Containers.Privileged, "Give the process extended Linux capabilities inside the container.  The default is false")
	flags.BoolVarP(&execOpts.Tty, "tty", "t", false, "Allocate a pseudo-TTY. The default is false")

	userFlagName := "user"
	flags.StringVarP(&execOpts.User, userFlagName, "u", "", "Sets the username or UID used and optionally the groupname or GID for the specified command")
	_ = cmd.RegisterFlagCompletionFunc(userFlagName, common.AutocompleteUserFlag)

	preserveFdsFlagName := "preserve-fds"
	flags.UintVar(&execOpts.PreserveFDs, preserveFdsFlagName, 0, "Pass N additional file descriptors to the container")
	_ = cmd.RegisterFlagCompletionFunc(preserveFdsFlagName, completion.AutocompleteNone)

	preserveFdFlagName := "preserve-fd"
	flags.UintSliceVar(&execOpts.PreserveFD, preserveFdFlagName, nil, "Pass a list of additional file descriptors to the container")
	_ = cmd.RegisterFlagCompletionFunc(preserveFdFlagName, completion.AutocompleteNone)

	workdirFlagName := "workdir"
	flags.StringVarP(&execOpts.WorkDir, workdirFlagName, "w", "", "Working directory inside the container")
	_ = cmd.RegisterFlagCompletionFunc(workdirFlagName, completion.AutocompleteDefault)

	waitFlagName := "wait"
	flags.Int32(waitFlagName, 0, "Total seconds to wait for container to start")
	_ = flags.MarkHidden(waitFlagName)

	if registry.IsRemote() {
		_ = flags.MarkHidden("preserve-fds")
	}
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: execCommand,
	})
	execFlags(execCommand)
	validate.AddLatestFlag(execCommand, &execOpts.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerExecCommand,
		Parent:  containerCmd,
	})
	execFlags(containerExecCommand)
	validate.AddLatestFlag(containerExecCommand, &execOpts.Latest)
}

func exec(cmd *cobra.Command, args []string) error {
	var nameOrID string

	if len(args) == 0 && !execOpts.Latest {
		return errors.New("exec requires the name or ID of a container or the --latest flag")
	}
	execOpts.Cmd = args
	if !execOpts.Latest {
		execOpts.Cmd = args[1:]
		nameOrID = strings.TrimPrefix(args[0], "/")
	}
	// Validate given environment variables
	execOpts.Envs = make(map[string]string)
	for _, f := range envFile {
		fileEnv, err := envLib.ParseFile(f)
		if err != nil {
			return err
		}
		execOpts.Envs = envLib.Join(execOpts.Envs, fileEnv)
	}

	cliEnv, err := envLib.ParseSlice(envInput)
	if err != nil {
		return fmt.Errorf("parsing environment variables: %w", err)
	}

	execOpts.Envs = envLib.Join(execOpts.Envs, cliEnv)

	for _, fd := range execOpts.PreserveFD {
		if !rootless.IsFdInherited(int(fd)) {
			return fmt.Errorf("file descriptor %d is not available - the preserve-fd option requires that file descriptors must be passed", fd)
		}
	}

	for fd := 3; fd < int(3+execOpts.PreserveFDs); fd++ {
		if !rootless.IsFdInherited(fd) {
			return fmt.Errorf("file descriptor %d is not available - the preserve-fds option requires that file descriptors must be passed", fd)
		}
	}

	if cmd.Flags().Changed("wait") {
		seconds, err := cmd.Flags().GetInt32("wait")
		if err != nil {
			return err
		}
		if err := execWait(nameOrID, seconds); err != nil {
			if errors.Is(err, define.ErrCanceled) {
				return fmt.Errorf("timed out waiting for container: %s", nameOrID)
			}
		}
	}

	if !execDetach {
		streams := define.AttachStreams{}
		streams.OutputStream = os.Stdout
		streams.ErrorStream = os.Stderr
		if execOpts.Interactive {
			streams.InputStream = bufio.NewReader(os.Stdin)
			streams.AttachInput = true
		}
		streams.AttachOutput = true
		streams.AttachError = true

		exitCode, err := registry.ContainerEngine().ContainerExec(registry.GetContext(), nameOrID, execOpts, streams)
		registry.SetExitCode(exitCode)
		return err
	}

	id, err := registry.ContainerEngine().ContainerExecDetached(registry.GetContext(), nameOrID, execOpts)
	if err != nil {
		return err
	}
	fmt.Println(id)
	return nil
}

func execWait(ctr string, seconds int32) error {
	maxDuration := time.Duration(seconds) * time.Second
	interval := 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(registry.Context(), maxDuration)
	defer cancel()

	waitOptions.Conditions = []string{define.ContainerStateRunning.String()}

	startTime := time.Now()
	for time.Since(startTime) < maxDuration {
		_, err := registry.ContainerEngine().ContainerWait(ctx, []string{ctr}, waitOptions)
		if err == nil {
			return nil
		}

		if !errors.Is(err, define.ErrNoSuchCtr) {
			return err
		}

		interval *= 2
		since := time.Since(startTime)
		if since+interval > maxDuration {
			interval = maxDuration - since
		}
		time.Sleep(interval)
	}
	return define.ErrCanceled
}
