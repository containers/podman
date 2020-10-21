package volumes

import (
	"fmt"
	"os"
	"text/template"

	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

var (
	volumeInspectDescription = `Display detailed information on one or more volumes.

  Use a Go template to change the format from JSON.`
	inspectCommand = &cobra.Command{
		Use:   "inspect [options] VOLUME [VOLUME...]",
		Short: "Display detailed information on one or more volumes",
		Long:  volumeInspectDescription,
		RunE:  inspect,
		Example: `podman volume inspect myvol
  podman volume inspect --all
  podman volume inspect --format "{{.Driver}} {{.Scope}}" myvol`,
	}
)

var (
	inspectOpts   = entities.VolumeInspectOptions{}
	inspectFormat string
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: inspectCommand,
		Parent:  volumeCmd,
	})
	flags := inspectCommand.Flags()
	flags.BoolVarP(&inspectOpts.All, "all", "a", false, "Inspect all volumes")
	flags.StringVarP(&inspectFormat, "format", "f", "json", "Format volume output using Go template")
}

func inspect(cmd *cobra.Command, args []string) error {
	if (inspectOpts.All && len(args) > 0) || (!inspectOpts.All && len(args) < 1) {
		return errors.New("provide one or more volume names or use --all")
	}
	responses, err := registry.ContainerEngine().VolumeInspect(context.Background(), args, inspectOpts)
	if err != nil {
		return err
	}

	switch {
	case report.IsJSON(inspectFormat), inspectFormat == "":
		jsonOut, err := json.MarshalIndent(responses, "", "     ")
		if err != nil {
			return errors.Wrapf(err, "error marshalling inspect JSON")
		}
		fmt.Println(string(jsonOut))
	default:
		row := "{{range . }}" + report.NormalizeFormat(inspectFormat) + "{{end}}"
		tmpl, err := template.New("volumeInspect").Parse(row)
		if err != nil {
			return err
		}
		return tmpl.Execute(os.Stdout, responses)
	}
	return nil
}
