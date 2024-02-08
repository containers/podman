package secrets

import (
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/spf13/cobra"
)

var (
	existsCmd = &cobra.Command{
		Use:               "exists SECRET",
		Short:             "Check if a secret exists in local storage",
		Long:              `If the named secret exists in local storage, podman secret exists exits with 0, otherwise the exit code will be 1.`,
		Args:              cobra.ExactArgs(1),
		RunE:              exists,
		ValidArgsFunction: common.AutocompleteSecrets,
		Example: `podman secret exists ID
  podman secret exists SECRET || podman secret create SECRET <secret source>`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: existsCmd,
		Parent:  secretCmd,
	})
}

func exists(cmd *cobra.Command, args []string) error {
	found, err := registry.ContainerEngine().SecretExists(registry.GetContext(), args[0])
	if err != nil {
		return err
	}
	if !found.Value {
		registry.SetExitCode(1)
	}
	return nil
}
