package artifact

import (
	"fmt"
	"os"

	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/utils"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/report"
)

type artifactInspectFormat struct {
	Digest   string                                                  `json:"Digest"`
	Manifest struct{ Annotations, ArtifactType, Config, Layers any } `json:"Manifest"`
	Name     string                                                  `json:"Name"`
}

var (
	inspectCmd = &cobra.Command{
		Use:               "inspect [options] ARTIFACT",
		Short:             "Inspect an OCI artifact",
		Long:              "Provide details on an OCI artifact",
		RunE:              inspect,
		ValidArgsFunction: common.AutocompleteArtifacts,
		Example: `podman artifact inspect quay.io/myimage/myartifact:latest
  podman artifact inspect --format "{{.Name}}" myartifact
  podman artifact inspect --format "{{.Digest}}" myartifact`,
	}
	inspectOpts = entities.InspectOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: inspectCmd,
		Parent:  artifactCmd,
	})

	flags := inspectCmd.Flags()
	formatFlagName := "format"
	flags.StringVarP(&inspectOpts.Format, formatFlagName, "f", "json", "Format the output to a Go template or json")
	_ = inspectCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&artifactInspectFormat{}))
}

func inspect(_ *cobra.Command, args []string) error {
	artifactOptions := entities.ArtifactInspectOptions{}

	l := len(args)
	if l != 1 {
		return fmt.Errorf("accepts 1 arg, received %d", l)
	}
	data, err := registry.ImageEngine().ArtifactInspect(registry.Context(), args[0], artifactOptions)
	if err != nil {
		return err
	}
	if err := print(data); err != nil {
		return err
	}
	return nil
}

func print(inspectData *entities.ArtifactInspectReport) error {
	var err error
	data := []any{inspectData}

	switch {
	case report.IsJSON(inspectOpts.Format) || inspectOpts.Format == "":
		err = utils.PrintGenericJSON(inspectData)
	default:
		// User has given a custom --format
		var rpt *report.Formatter
		rpt, err = report.New(os.Stdout, "inspect").Parse(report.OriginUser, inspectOpts.Format)
		if err != nil {
			return err
		}
		defer rpt.Flush()

		err = rpt.Execute(data)
	}

	return err
}
