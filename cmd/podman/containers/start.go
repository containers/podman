package containers

import (
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/cmd/podman/utils"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	startDescription = `Starts one or more containers.  The container name or ID can be used.`
	startCommand     = &cobra.Command{
		Use:   "start [flags] CONTAINER [CONTAINER...]",
		Short: "Start one or more containers",
		Long:  startDescription,
		RunE:  start,
		Example: `podman start --latest
  podman start 860a4b231279 5421ab43b45
  podman start --interactive --attach imageID`,
	}

	containerStartCommand = &cobra.Command{
		Use:   startCommand.Use,
		Short: startCommand.Short,
		Long:  startCommand.Long,
		RunE:  startCommand.RunE,
		Example: `podman container start --latest
  podman container start 860a4b231279 5421ab43b45
  podman container start --interactive --attach imageID`,
	}
)

var (
	startOptions entities.ContainerStartOptions
)

func startFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&startOptions.Attach, "attach", "a", false, "Attach container's STDOUT and STDERR")
	flags.StringVar(&startOptions.DetachKeys, "detach-keys", containerConfig.DetachKeys(), "Select the key sequence for detaching a container. Format is a single character `[a-Z]` or a comma separated sequence of `ctrl-<value>`, where `<value>` is one of: `a-z`, `@`, `^`, `[`, `\\`, `]`, `^` or `_`")
	flags.BoolVarP(&startOptions.Interactive, "interactive", "i", false, "Keep STDIN open even if not attached")
	flags.BoolVarP(&startOptions.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.BoolVar(&startOptions.SigProxy, "sig-proxy", false, "Proxy received signals to the process (default true if attaching, false otherwise)")
	if registry.IsRemote() {
		_ = flags.MarkHidden("latest")
		_ = flags.MarkHidden("sig-proxy")
	}
}
func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: startCommand,
	})
	flags := startCommand.Flags()
	startFlags(flags)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: containerStartCommand,
		Parent:  containerCmd,
	})

	containerStartFlags := containerStartCommand.Flags()
	startFlags(containerStartFlags)
}

func start(cmd *cobra.Command, args []string) error {
	var errs utils.OutputErrors
	if len(args) == 0 && !startOptions.Latest {
		return errors.New("start requires at least one argument")
	}
	if len(args) > 1 && startOptions.Attach {
		return errors.Errorf("you cannot start and attach multiple containers at once")
	}

	sigProxy := startOptions.SigProxy || startOptions.Attach
	if cmd.Flag("sig-proxy").Changed {
		sigProxy = startOptions.SigProxy
	}

	if sigProxy && !startOptions.Attach {
		return errors.Wrapf(define.ErrInvalidArg, "you cannot use sig-proxy without --attach")
	}
	if startOptions.Attach {
		startOptions.Stdin = os.Stdin
		startOptions.Stderr = os.Stderr
		startOptions.Stdout = os.Stdout
	}

	responses, err := registry.ContainerEngine().ContainerStart(registry.GetContext(), args, startOptions)
	if err != nil {
		return err
	}

	for _, r := range responses {
		if r.Err == nil {
			fmt.Println(r.RawInput)
		} else {
			errs = append(errs, r.Err)
		}
	}
	// TODO need to understand an implement exitcodes
	return errs.PrintErrors()
}
