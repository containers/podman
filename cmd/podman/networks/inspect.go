package network

import (
	"encoding/json"
	"fmt"

	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	networkinspectDescription = `Inspect network`
	networkinspectCommand     = &cobra.Command{
		Use:     "inspect NETWORK [NETWORK...] [flags] ",
		Short:   "network inspect",
		Long:    networkinspectDescription,
		RunE:    networkInspect,
		Example: `podman network inspect podman`,
		Args:    cobra.MinimumNArgs(1),
		Annotations: map[string]string{
			registry.ParentNSRequired: "",
		},
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: networkinspectCommand,
		Parent:  networkCmd,
	})
}

func networkInspect(cmd *cobra.Command, args []string) error {
	responses, err := registry.ContainerEngine().NetworkInspect(registry.Context(), args, entities.NetworkInspectOptions{})
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(responses, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
