package pods

import (
	"context"
	"errors"
	"os"
	"text/template"

	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	inspectOptions = entities.PodInspectOptions{}
)

var (
	inspectDescription = `Display the configuration for a pod by name or id

	By default, this will render all results in a JSON array.`

	inspectCmd = &cobra.Command{
		Use:               "inspect [options] POD [POD...]",
		Short:             "Displays a pod configuration",
		Long:              inspectDescription,
		RunE:              inspect,
		ValidArgsFunction: common.AutocompletePods,
		Example:           `podman pod inspect podID`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: inspectCmd,
		Parent:  podCmd,
	})
	flags := inspectCmd.Flags()

	formatFlagName := "format"
	flags.StringVarP(&inspectOptions.Format, formatFlagName, "f", "json", "Format the output to a Go template or json")
	_ = inspectCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&entities.PodInspectReport{}))

	validate.AddLatestFlag(inspectCmd, &inspectOptions.Latest)
}

func inspect(cmd *cobra.Command, args []string) error {
	if len(args) < 1 && !inspectOptions.Latest {
		return errors.New("you must provide the name or id of a running pod")
	}
	if len(args) > 0 && inspectOptions.Latest {
		return errors.New("--latest and containers cannot be used together")
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

	// Cannot use report.New() as it enforces {{range .}} for OriginUser templates
	tmpl := template.New(cmd.Name()).Funcs(template.FuncMap(report.DefaultFuncs))
	format := report.NormalizeFormat(inspectOptions.Format)
	tmpl, err = tmpl.Parse(format)
	if err != nil {
		return err
	}
	return tmpl.Execute(os.Stdout, *responses)
}
