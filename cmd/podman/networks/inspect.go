package network

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"text/template"

	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	networkinspectDescription = `Inspect network`
	networkinspectCommand     = &cobra.Command{
		Use:     "inspect [options] NETWORK [NETWORK...]",
		Short:   "network inspect",
		Long:    networkinspectDescription,
		RunE:    networkInspect,
		Example: `podman network inspect podman`,
		Args:    cobra.MinimumNArgs(1),
	}
)

var (
	networkInspectOptions entities.NetworkInspectOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: networkinspectCommand,
		Parent:  networkCmd,
	})
	flags := networkinspectCommand.Flags()
	flags.StringVarP(&networkInspectOptions.Format, "format", "f", "", "Pretty-print network to JSON or using a Go template")
}

func networkInspect(_ *cobra.Command, args []string) error {
	responses, err := registry.ContainerEngine().NetworkInspect(registry.Context(), args, entities.NetworkInspectOptions{})
	if err != nil {
		return err
	}

	switch {
	case report.IsJSON(networkInspectOptions.Format) || networkInspectOptions.Format == "":
		b, err := json.MarshalIndent(responses, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	default:
		row := report.NormalizeFormat(networkInspectOptions.Format)
		// There can be more than 1 in the inspect output.
		row = "{{range . }}" + row + "{{end}}"
		tmpl, err := template.New("inspectNetworks").Parse(row)
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 8, 2, 0, ' ', 0)
		defer w.Flush()

		return tmpl.Execute(w, responses)
	}
	return nil
}
