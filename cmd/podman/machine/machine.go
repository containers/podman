package machine

import (
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/validate"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	noOp = func(cmd *cobra.Command, args []string) error {
		return nil
	}
	// Command: podman _machine_
	machineCmd = &cobra.Command{
		Use:                "machine",
		Short:              "Manage a virtual machine",
		Long:               "Manage a virtual machine. Virtual machines are used to run Podman on Macs.",
		PersistentPreRunE:  noOp,
		PersistentPostRunE: noOp,
		RunE:               validate.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: machineCmd,
	})
}
