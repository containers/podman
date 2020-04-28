package images

import (
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	// Command: podman _network_
	cmd = &cobra.Command{
		Use:              "network",
		Short:            "Manage networks",
		Long:             "Manage networks",
		TraverseChildren: true,
		RunE:             registry.SubCommandExists,
	}
)

// TODO add the following to main.go to get networks back onto the
// command list.
//_ "github.com/containers/libpod/cmd/podman/networks"
func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: cmd,
	})
}
