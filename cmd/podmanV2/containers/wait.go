package containers

import (
	"context"
	"fmt"
	"time"

	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	waitDescription = `Block until one or more containers stop and then print their exit codes.
`
	waitCommand = &cobra.Command{
		Use:   "wait [flags] CONTAINER [CONTAINER...]",
		Short: "Block on one or more containers",
		Long:  waitDescription,
		RunE:  wait,
		Example: `podman wait --latest
  podman wait --interval 5000 ctrID
  podman wait ctrID1 ctrID2`,
	}
)

var waitFlags = entities.WaitOptions{}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: waitCommand,
		Parent:  containerCmd,
	})

	waitCommand.SetHelpTemplate(registry.HelpTemplate())
	waitCommand.SetUsageTemplate(registry.UsageTemplate())
	flags := waitCommand.Flags()
	flags.DurationVarP(&waitFlags.Interval, "interval", "i", time.Duration(250), "Milliseconds to wait before polling for completion")
	flags.BoolVarP(&waitFlags.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	flags.StringVar(&waitFlags.Condition, "condition", "stopped", "Condition to wait on")
}

func wait(cmd *cobra.Command, args []string) error {
	if waitFlags.Latest && len(args) > 0 {
		return errors.New("cannot combine latest flag and arguments")
	}
	if waitFlags.Interval == 0 {
		return errors.New("interval must be greater then 0")
	}

	responses, err := registry.ContainerEngine().ContainerWait(context.Background(), args, waitFlags)
	if err != nil {
		return err
	}
	for _, r := range responses {
		if r.Error == nil {
			fmt.Println(r.Id)
		}
	}
	for _, r := range responses {
		if r.Error != nil {
			fmt.Println(err)
		}
	}
	return nil
}
