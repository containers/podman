package volumes

import (
	"fmt"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/utils"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/spf13/cobra"
)

var (
	reloadDescription = `Check all configured volume plugins and update the libpod database with all available volumes.

  Existing volumes are also removed from the database when they are no longer present in the plugin.`
	reloadCommand = &cobra.Command{
		Use:               "reload",
		Args:              validate.NoArgs,
		Short:             "Reload all volumes from volume plugins",
		Long:              reloadDescription,
		RunE:              reload,
		ValidArgsFunction: completion.AutocompleteNone,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: reloadCommand,
		Parent:  volumeCmd,
	})
}

func reload(cmd *cobra.Command, args []string) error {
	report, err := registry.ContainerEngine().VolumeReload(registry.Context())
	if err != nil {
		return err
	}
	printReload("Added", report.Added)
	printReload("Removed", report.Removed)
	errs := (utils.OutputErrors)(report.Errors)
	return errs.PrintErrors()
}

func printReload(typ string, values []string) {
	if len(values) > 0 {
		fmt.Println(typ + ":")
		for _, name := range values {
			fmt.Println(name)
		}
	}
}
