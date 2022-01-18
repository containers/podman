package network

import (
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/spf13/cobra"
)

var (
	networkExistsDescription = `If the named network exists, podman network exists exits with 0, otherwise the exit code will be 1.`
	networkExistsCommand     = &cobra.Command{
		Use:               "exists NETWORK",
		Short:             "network exists",
		Long:              networkExistsDescription,
		RunE:              networkExists,
		Example:           `podman network exists net1`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: common.AutocompleteNetworks,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: networkExistsCommand,
		Parent:  networkCmd,
	})
}

func networkExists(cmd *cobra.Command, args []string) error {
	response, err := registry.ContainerEngine().NetworkExists(registry.GetContext(), args[0])
	if err != nil {
		return err
	}
	if !response.Value {
		registry.SetExitCode(1)
	}
	return nil
}
