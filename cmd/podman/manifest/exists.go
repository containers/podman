package manifest

import (
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/spf13/cobra"
)

var (
	existsCmd = &cobra.Command{
		Use:               "exists MANIFEST",
		Short:             "Check if a manifest list exists in local storage",
		Long:              `If the manifest list exists in local storage, podman manifest exists exits with 0, otherwise the exit code will be 1.`,
		Args:              cobra.ExactArgs(1),
		RunE:              exists,
		ValidArgsFunction: common.AutocompleteImages,
		Example:           "podman manifest exists mylist",
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: existsCmd,
		Parent:  manifestCmd,
	})
}

func exists(cmd *cobra.Command, args []string) error {
	found, err := registry.ImageEngine().ManifestExists(registry.GetContext(), args[0])
	if err != nil {
		return err
	}
	if !found.Value {
		registry.SetExitCode(1)
	}
	return nil
}
