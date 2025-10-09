package artifact

import (
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/inspect"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	inspectCmd = &cobra.Command{
		Use:               "inspect [ARTIFACT...]",
		Short:             "Inspect an OCI artifact",
		Long:              "Provide details on an OCI artifact",
		RunE:              artifactInspect,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: common.AutocompleteArtifacts,
		Example:           `podman artifact inspect quay.io/myimage/myartifact:latest`,
	}
	inspectOpts *entities.InspectOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: inspectCmd,
		Parent:  artifactCmd,
	})

	inspectOpts = new(entities.InspectOptions)

	flags := inspectCmd.Flags()
	formatFlagName := "format"
	flags.StringVarP(&inspectOpts.Format, formatFlagName, "f", "json", "Format volume output using JSON or a Go template")

	// This is something we wanted to do but did not seem important enough for initial PR
	// remoteFlagName := "remote"
	// flags.BoolVar(&inspectFlag.remote, remoteFlagName, false, "Inspect the image on a container image registry")

	// TODO When the inspect structure has been defined, we need to uncomment and redirect this.  Reminder, this
	// will also need to be reflected in the podman-artifact-inspect man page
	_ = inspectCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&entities.ArtifactInspectReport{}))
}

func artifactInspect(_ *cobra.Command, args []string) error {
	inspectOpts.Type = common.ArtifactType
	return inspect.Inspect(args, *inspectOpts)
}
