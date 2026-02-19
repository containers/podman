//go:build remote

package images

import (
	"fmt"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/spf13/cobra"
)

var (
	versionDescription = `Print build version`
	versionCmd         = &cobra.Command{
		Use:               "version",
		Args:              validate.NoArgs,
		Short:             "Print build version",
		Long:              versionDescription,
		RunE:              version,
		ValidArgsFunction: completion.AutocompleteNone,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: versionCmd,
		Parent:  buildxCmd,
	})
}

func version(cmd *cobra.Command, args []string) error {
	info, err := registry.ContainerEngine().Info(registry.GetContext())
	if err != nil {
		return err
	}
	fmt.Printf("buildah %s\n", info.Host.BuildahVersion)
	return nil
}
