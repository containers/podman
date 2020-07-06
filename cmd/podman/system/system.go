package system

import (
	"github.com/containers/libpod/v2/cmd/podman/registry"
	"github.com/containers/libpod/v2/cmd/podman/validate"
	"github.com/containers/libpod/v2/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// Pull in configured json library
	json = registry.JSONLibrary()

	// Command: podman _system_
	systemCmd = &cobra.Command{
		Use:              "system",
		Short:            "Manage podman",
		Long:             "Manage podman",
		TraverseChildren: true,
		RunE:             validate.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: systemCmd,
	})
}
