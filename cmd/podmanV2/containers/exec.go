package containers

import (
	"bufio"
	"os"

	"github.com/containers/libpod/cmd/podmanV2/common"
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	envLib "github.com/containers/libpod/pkg/env"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	execDescription = `Execute the specified command inside a running container.
`
	execCommand = &cobra.Command{
		Use:     "exec [flags] CONTAINER [COMMAND [ARG...]]",
		Short:   "Run a process in a running container",
		Long:    execDescription,
		PreRunE: preRunE,
		RunE:    exec,
		Example: `podman exec -it ctrID ls
  podman exec -it -w /tmp myCtr pwd
  podman exec --user root ctrID ls`,
	}
)

var (
	envInput, envFile []string
	execOpts          entities.ExecOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: execCommand,
	})
	flags := execCommand.Flags()
	flags.SetInterspersed(false)
	flags.StringVar(&execOpts.DetachKeys, "detach-keys", common.GetDefaultDetachKeys(), "Select the key sequence for detaching a container. Format is a single character [a-Z] or ctrl-<value> where <value> is one of: a-z, @, ^, [, , or _")
	flags.StringArrayVarP(&envInput, "env", "e", []string{}, "Set environment variables")
	flags.StringSliceVar(&envFile, "env-file", []string{}, "Read in a file of environment variables")
	flags.BoolVarP(&execOpts.Interactive, "interactive", "i", false, "Keep STDIN open even if not attached")
	flags.BoolVarP(&execOpts.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.BoolVar(&execOpts.Privileged, "privileged", false, "Give the process extended Linux capabilities inside the container.  The default is false")
	flags.BoolVarP(&execOpts.Tty, "tty", "t", false, "Allocate a pseudo-TTY. The default is false")
	flags.StringVarP(&execOpts.User, "user", "u", "", "Sets the username or UID used and optionally the groupname or GID for the specified command")
	flags.UintVar(&execOpts.PreserveFDs, "preserve-fds", 0, "Pass N additional file descriptors to the container")
	flags.StringVarP(&execOpts.WorkDir, "workdir", "w", "", "Working directory inside the container")
	if registry.IsRemote() {
		_ = flags.MarkHidden("latest")
		_ = flags.MarkHidden("preserve-fds")
	}

}
func exec(cmd *cobra.Command, args []string) error {
	var nameOrId string
	execOpts.Cmd = args
	if !execOpts.Latest {
		execOpts.Cmd = args[1:]
		nameOrId = args[0]
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
		return errors.Wrap(err, "error parsing environment variables")
	}

	execOpts.Envs = envLib.Join(execOpts.Envs, cliEnv)
	execOpts.Streams.OutputStream = os.Stdout
	execOpts.Streams.ErrorStream = os.Stderr
	if execOpts.Interactive {
		execOpts.Streams.InputStream = bufio.NewReader(os.Stdin)
		execOpts.Streams.AttachInput = true
	}
	execOpts.Streams.AttachOutput = true
	execOpts.Streams.AttachError = true

	exitCode, err := registry.ContainerEngine().ContainerExec(registry.GetContext(), nameOrId, execOpts)
	registry.SetExitCode(exitCode)
	return err
}
