package network

import (
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/cmd/podman/validate"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// Command: podman _network_
	networkCmd = &cobra.Command{
		Use:              "network",
		Short:            "Manage networks",
		Long:             "Manage networks",
		TraverseChildren: true,
		RunE:             validate.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: networkCmd,
	})
}
