package pods

import (
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// Command: podman _pod_
	podCmd = &cobra.Command{
		Use:              "pod",
		Short:            "Manage pods",
		Long:             "Manage pods",
		TraverseChildren: true,
		RunE:             registry.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: podCmd,
	})
}
