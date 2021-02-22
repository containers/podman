package images

import (
	"strings"

	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	rmiCmd = &cobra.Command{
		Use:               strings.Replace(rmCmd.Use, "rm ", "rmi ", 1),
		Args:              rmCmd.Args,
		Short:             rmCmd.Short,
		Long:              rmCmd.Long,
		RunE:              rmCmd.RunE,
		ValidArgsFunction: rmCmd.ValidArgsFunction,
		Example:           strings.Replace(rmCmd.Example, "podman image rm", "podman rmi", -1),
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: rmiCmd,
	})
	imageRemoveFlagSet(rmiCmd.Flags())
}
