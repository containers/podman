package system

import (
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// Command: podman _system_
	systemCmd = &cobra.Command{
		Use:              "system",
		Short:            "Manage podman",
		Long:             "Manage podman",
		TraverseChildren: true,
		RunE:             registry.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: systemCmd,
	})
}
