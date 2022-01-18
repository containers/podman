package manifest

import (
	"fmt"

	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/spf13/cobra"
)

var (
	inspectCmd = &cobra.Command{
		Use:               "inspect IMAGE",
		Short:             "Display the contents of a manifest list or image index",
		Long:              "Display the contents of a manifest list or image index.",
		RunE:              inspect,
		ValidArgsFunction: common.AutocompleteImages,
		Example:           "podman manifest inspect localhost/list",
		Args:              cobra.ExactArgs(1),
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: inspectCmd,
		Parent:  manifestCmd,
	})
}

func inspect(cmd *cobra.Command, args []string) error {
	buf, err := registry.ImageEngine().ManifestInspect(registry.Context(), args[0])
	if err != nil {
		return err
	}
	fmt.Println(string(buf))
	return nil
}
