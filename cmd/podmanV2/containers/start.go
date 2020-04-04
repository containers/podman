package containers

import (
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podmanV2/common"
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/cmd/podmanV2/utils"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	startDescription = `Starts one or more containers.  The container name or ID can be used.`
	startCommand     = &cobra.Command{
		Use:     "start [flags] CONTAINER [CONTAINER...]",
		Short:   "Start one or more containers",
		Long:    startDescription,
		RunE:    start,
		PreRunE: preRunE,
		Args:    cobra.MinimumNArgs(1),
		Example: `podman start --latest
  podman start 860a4b231279 5421ab43b45
  podman start --interactive --attach imageID`,
	}
)

var (
	startOptions entities.ContainerStartOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: startCommand,
	})
	flags := startCommand.Flags()
	flags.BoolVarP(&startOptions.Attach, "attach", "a", false, "Attach container's STDOUT and STDERR")
	flags.StringVar(&startOptions.DetachKeys, "detach-keys", common.GetDefaultDetachKeys(), "Select the key sequence for detaching a container. Format is a single character `[a-Z]` or a comma separated sequence of `ctrl-<value>`, where `<value>` is one of: `a-z`, `@`, `^`, `[`, `\\`, `]`, `^` or `_`")
	flags.BoolVarP(&startOptions.Interactive, "interactive", "i", false, "Keep STDIN open even if not attached")
	flags.BoolVarP(&startOptions.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.BoolVar(&startOptions.SigProxy, "sig-proxy", false, "Proxy received signals to the process (default true if attaching, false otherwise)")
	if registry.IsRemote() {
		_ = flags.MarkHidden("latest")
		_ = flags.MarkHidden("sig-proxy")
	}

}

func start(cmd *cobra.Command, args []string) error {
	var errs utils.OutputErrors
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
			fmt.Println(r.Id)
		} else {
			errs = append(errs, r.Err)
		}
	}
	// TODO need to understand an implement exitcodes
	return errs.PrintErrors()
}
