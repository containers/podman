package healthcheck

import (
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/cmd/podman/validate"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// Command: healthcheck
	healthCmd = &cobra.Command{
		Use:              "healthcheck",
		Short:            "Manage Healthcheck",
		Long:             "Manage Healthcheck",
		TraverseChildren: true,
		RunE:             validate.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: healthCmd,
	})
}
