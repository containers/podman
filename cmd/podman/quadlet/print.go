package quadlet

import (
	"fmt"

	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/spf13/cobra"
)

var (
	quadletPrintDescription = `Print the contents of a Quadlet, displaying the file including all comments`

	quadletPrintCmd = &cobra.Command{
		Use:               "print QUADLET",
		Short:             "Display the contents of a quadlet",
		Long:              quadletPrintDescription,
		RunE:              print,
		ValidArgsFunction: common.AutocompleteQuadlets,
		Args:              cobra.ExactArgs(1),
		Example: `podman quadlet print myquadlet.container
podman quadlet print mypod.pod
podman quadlet print myimage.build`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: quadletPrintCmd,
		Parent:  quadletCmd,
	})
}

func print(cmd *cobra.Command, args []string) error {
	quadletContents, err := registry.ContainerEngine().QuadletPrint(registry.Context(), args[0])
	if err != nil {
		return err
	}

	fmt.Print(quadletContents)

	return nil
}
