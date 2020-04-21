package pods

import (
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/util"
	"github.com/spf13/cobra"
)

var (
	// Pull in configured json library
	json = registry.JsonLibrary()

	// Command: podman _pod_
	podCmd = &cobra.Command{
		Use:              "pod",
		Short:            "Manage pods",
		Long:             "Manage pods",
		TraverseChildren: true,
		RunE:             registry.SubCommandExists,
	}
	containerConfig = util.DefaultContainerConfig()
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: podCmd,
	})
}
