package artifact

import (
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/utils"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	inspectCmd = &cobra.Command{
		Use:               "inspect [ARTIFACT...]",
		Short:             "Inspect an OCI artifact",
		Long:              "Provide details on an OCI artifact",
		RunE:              inspect,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: common.AutocompleteArtifacts,
		Example:           `podman artifact inspect quay.io/myimage/myartifact:latest`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: inspectCmd,
		Parent:  artifactCmd,
	})

	// TODO When things firm up on inspect looks, we can do a format implementation
	// flags := inspectCmd.Flags()
	// formatFlagName := "format"
	// flags.StringVar(&inspectFlag.format, formatFlagName, "", "Format volume output using JSON or a Go template")

	// This is something we wanted to do but did not seem important enough for initial PR
	// remoteFlagName := "remote"
	// flags.BoolVar(&inspectFlag.remote, remoteFlagName, false, "Inspect the image on a container image registry")

	// TODO When the inspect structure has been defined, we need to uncomment and redirect this.  Reminder, this
	// will also need to be reflected in the podman-artifact-inspect man page
	// _ = inspectCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&machine.InspectInfo{}))
}

func inspect(_ *cobra.Command, args []string) error {
	artifactOptions := entities.ArtifactInspectOptions{}

	var prevErr error

	for _, name := range args {
		data, err := registry.ImageEngine().ArtifactInspect(registry.Context(), name, artifactOptions)
		if err != nil {
			if prevErr != nil {
				logrus.Error(prevErr)
			}
			prevErr = err
			continue
		}
		// Print all successful inspections
		if err := utils.PrintGenericJSON(data); err != nil {
			if prevErr != nil {
				logrus.Error(prevErr)
			}
			prevErr = err
		}
	}

	return prevErr
}
