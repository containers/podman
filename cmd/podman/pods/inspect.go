package pods

import (
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/inspect"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	inspectDescription = `Display the configuration for a pod by name or id

	By default, this will render all results in a JSON array.`

	inspectCmd = &cobra.Command{
		Use:               "inspect [options] POD [POD...]",
		Short:             "Displays a pod configuration",
		Long:              inspectDescription,
		RunE:              inspectExec,
		ValidArgsFunction: common.AutocompletePods,
		Example:           `podman pod inspect podID`,
	}

	inspectOpts = &entities.InspectOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: inspectCmd,
		Parent:  podCmd,
	})
	flags := inspectCmd.Flags()

	formatFlagName := "format"
	flags.StringVarP(&inspectOpts.Format, formatFlagName, "f", "json", "Format the output to a Go template or json")
	_ = inspectCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&entities.PodInspectReport{}))

	validate.AddLatestFlag(inspectCmd, &inspectOpts.Latest)
}

func inspectExec(cmd *cobra.Command, args []string) error {
	// We need backwards compat with the old podman pod inspect behavior.
	// https://github.com/containers/podman/pull/15675
	inspectOpts.Type = common.PodLegacyType
	return inspect.Inspect(args, *inspectOpts)
}
