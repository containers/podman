package volumes

import (
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// Pull in configured json library
	json = registry.JsonLibrary()

	// Command: podman _volume_
	volumeCmd = &cobra.Command{
		Use:              "volume",
		Short:            "Manage volumes",
		Long:             "Volumes are created in and can be shared between containers",
		TraverseChildren: true,
		RunE:             registry.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: volumeCmd,
	})
}
