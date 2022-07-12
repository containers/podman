package containers

import (
	"bufio"
	"errors"
	"fmt"
	"os"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	envLib "github.com/containers/podman/v4/pkg/env"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/spf13/cobra"
)

var (
	execDescription = `Execute the specified command inside a running container.
`
	execCommand = &cobra.Command{
		Use:               "exec [options] CONTAINER [COMMAND [ARG...]]",
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
	flags.StringSliceVar(&envFile, envFileFlagName, []string{}, "Read in a file of environment variables")
	_ = cmd.RegisterFlagCompletionFunc(envFileFlagName, completion.AutocompleteDefault)

	flags.BoolVarP(&execOpts.Interactive, "interactive", "i", false, "Keep STDIN open even if not attached")
	flags.BoolVar(&execOpts.Privileged, "privileged", false, "Give the process extended Linux capabilities inside the container.  The default is false")
	flags.BoolVarP(&execOpts.Tty, "tty", "t", false, "Allocate a pseudo-TTY. The default is false")

	userFlagName := "user"
	flags.StringVarP(&execOpts.User, userFlagName, "u", "", "Sets the username or UID used and optionally the groupname or GID for the specified command")
	_ = cmd.RegisterFlagCompletionFunc(userFlagName, common.AutocompleteUserFlag)

	preserveFdsFlagName := "preserve-fds"
	flags.UintVar(&execOpts.PreserveFDs, preserveFdsFlagName, 0, "Pass N additional file descriptors to the container")
	_ = cmd.RegisterFlagCompletionFunc(preserveFdsFlagName, completion.AutocompleteNone)

	workdirFlagName := "workdir"
	flags.StringVarP(&execOpts.WorkDir, workdirFlagName, "w", "", "Working directory inside the container")
	_ = cmd.RegisterFlagCompletionFunc(workdirFlagName, completion.AutocompleteDefault)

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

func exec(_ *cobra.Command, args []string) error {
	var nameOrID string

	if len(args) == 0 && !execOpts.Latest {
		return errors.New("exec requires the name or ID of a container or the --latest flag")
	}
	execOpts.Cmd = args
	if !execOpts.Latest {
		execOpts.Cmd = args[1:]
		nameOrID = args[0]
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
		return fmt.Errorf("error parsing environment variables: %w", err)
	}

	execOpts.Envs = envLib.Join(execOpts.Envs, cliEnv)

	for fd := 3; fd < int(3+execOpts.PreserveFDs); fd++ {
		if !rootless.IsFdInherited(fd) {
			return fmt.Errorf("file descriptor %d is not available - the preserve-fds option requires that file descriptors must be passed", fd)
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
