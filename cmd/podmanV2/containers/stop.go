package containers

import (
	"context"
	"fmt"

	"github.com/containers/libpod/cmd/podmanV2/parse"
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/cmd/podmanV2/utils"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	stopDescription = `Stops one or more running containers.  The container name or ID can be used.

  A timeout to forcibly stop the container can also be set but defaults to 10 seconds otherwise.`
	stopCommand = &cobra.Command{
		Use:               "stop [flags] CONTAINER [CONTAINER...]",
		Short:             "Stop one or more containers",
		Long:              stopDescription,
		RunE:              stop,
		PersistentPreRunE: preRunE,
		Args: func(cmd *cobra.Command, args []string) error {
			return parse.CheckAllLatestAndCIDFile(cmd, args, false, true)
		},
		Example: `podman stop ctrID
  podman stop --latest
  podman stop --timeout 2 mywebserver 6e534f14da9d`,
	}
)

var (
	stopOptions = entities.StopOptions{}
	stopTimeout uint
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: stopCommand,
	})
	flags := stopCommand.Flags()
	flags.BoolVarP(&stopOptions.All, "all", "a", false, "Stop all running containers")
	flags.BoolVarP(&stopOptions.Ignore, "ignore", "i", false, "Ignore errors when a specified container is missing")
	flags.StringArrayVarP(&stopOptions.CIDFiles, "cidfile", "", nil, "Read the container ID from the file")
	flags.BoolVarP(&stopOptions.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.UintVar(&stopTimeout, "time", define.CtrRemoveTimeout, "Seconds to wait for stop before killing the container")
	flags.UintVarP(&stopTimeout, "timeout", "t", define.CtrRemoveTimeout, "Seconds to wait for stop before killing the container")
	if registry.EngineOptions.EngineMode == entities.ABIMode {
		_ = flags.MarkHidden("latest")
		_ = flags.MarkHidden("cidfile")
		_ = flags.MarkHidden("ignore")
	}
}

func stop(cmd *cobra.Command, args []string) error {
	var (
		errs utils.OutputErrors
	)
	if cmd.Flag("timeout").Changed && cmd.Flag("time").Changed {
		return errors.New("the --timeout and --time flags are mutually exclusive")
	}
	stopOptions.Timeout = define.CtrRemoveTimeout
	if cmd.Flag("timeout").Changed || cmd.Flag("time").Changed {
		stopOptions.Timeout = stopTimeout
	}

	// TODO How do we access global attributes?
	//if c.Bool("trace") {
	//	span, _ := opentracing.StartSpanFromContext(Ctx, "stopCmd")
	//	defer span.Finish()
	//}
	responses, err := registry.ContainerEngine().ContainerStop(context.Background(), args, stopOptions)
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
	return errs.PrintErrors()
}
