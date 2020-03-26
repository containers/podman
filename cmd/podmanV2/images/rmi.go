package images

import (
	"strings"

	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	rmiCmd = &cobra.Command{
		Use:     strings.Replace(rmCmd.Use, "rm ", "rmi ", 1),
		Args:    rmCmd.Args,
		Short:   rmCmd.Short,
		Long:    rmCmd.Long,
		PreRunE: rmCmd.PreRunE,
		RunE:    rmCmd.RunE,
		Example: strings.Replace(rmCmd.Example, "podman image rm", "podman rmi", -1),
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: rmiCmd,
	})
	rmiCmd.SetHelpTemplate(registry.HelpTemplate())
	rmiCmd.SetUsageTemplate(registry.UsageTemplate())
}
