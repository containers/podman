package pods

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"text/template"

	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/validate"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	inspectOptions = entities.PodInspectOptions{}
)

var (
	inspectDescription = fmt.Sprintf(`Display the configuration for a pod by name or id

	By default, this will render all results in a JSON array.`)

	inspectCmd = &cobra.Command{
		Use:     "inspect [options] POD [POD...]",
		Short:   "Displays a pod configuration",
		Long:    inspectDescription,
		RunE:    inspect,
		Example: `podman pod inspect podID`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: inspectCmd,
		Parent:  podCmd,
	})
	flags := inspectCmd.Flags()
	flags.StringVarP(&inspectOptions.Format, "format", "f", "json", "Format the output to a Go template or json")
	validate.AddLatestFlag(inspectCmd, &inspectOptions.Latest)
}

func inspect(cmd *cobra.Command, args []string) error {

	if len(args) < 1 && !inspectOptions.Latest {
		return errors.Errorf("you must provide the name or id of a running pod")
	}
	if len(args) > 0 && inspectOptions.Latest {
		return errors.Errorf("--latest and containers cannot be used together")
	}

	if !inspectOptions.Latest {
		inspectOptions.NameOrID = args[0]
	}
	responses, err := registry.ContainerEngine().PodInspect(context.Background(), inspectOptions)
	if err != nil {
		return err
	}

	if report.IsJSON(inspectOptions.Format) {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "     ")
		return enc.Encode(responses)
	}

	row := report.NormalizeFormat(inspectOptions.Format)

	t, err := template.New("pod inspect").Parse(row)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 8, 2, 2, ' ', 0)
	return t.Execute(w, *responses)
}
