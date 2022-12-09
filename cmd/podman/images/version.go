package images

import (
	"fmt"

	"github.com/containers/buildah/define"
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
	fmt.Printf("%s %s\n", define.Package, define.Version)
	return nil
}
