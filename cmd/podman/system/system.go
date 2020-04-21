package system

import (
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// Pull in configured json library
	json = registry.JsonLibrary()

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
