package network

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/validate"
	"github.com/containers/podman/v2/libpod/network"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	networklistDescription = `List networks`
	networklistCommand     = &cobra.Command{
		Use:     "ls [options]",
		Args:    validate.NoArgs,
		Short:   "network list",
		Long:    networklistDescription,
		RunE:    networkList,
		Example: `podman network list`,
	}
)

var (
	networkListOptions entities.NetworkListOptions
	headers            = "NAME\tVERSION\tPLUGINS\n"
	defaultListRow     = "{{.Name}}\t{{.Version}}\t{{.Plugins}}\n"
)

func networkListFlags(flags *pflag.FlagSet) {
	// TODO enable filters based on something
	// flags.StringSliceVarP(&networklistCommand.Filter, "filter", "f",  []string{}, "Pause all running containers")
	flags.StringVarP(&networkListOptions.Format, "format", "f", "", "Pretty-print networks to JSON or using a Go template")
	flags.BoolVarP(&networkListOptions.Quiet, "quiet", "q", false, "display only names")
	flags.StringVarP(&networkListOptions.Filter, "filter", "", "", "Provide filter values (e.g. 'name=podman')")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: networklistCommand,
		Parent:  networkCmd,
	})
	flags := networklistCommand.Flags()
	networkListFlags(flags)
}

func networkList(cmd *cobra.Command, args []string) error {
	// validate the filter pattern.
	if len(networkListOptions.Filter) > 0 {
		tokens := strings.Split(networkListOptions.Filter, "=")
		if len(tokens) != 2 {
			return fmt.Errorf("invalid filter syntax : %s", networkListOptions.Filter)
		}
	}

	responses, err := registry.ContainerEngine().NetworkList(registry.Context(), networkListOptions)
	if err != nil {
		return err
	}

	// quiet means we only print the network names
	if networkListOptions.Quiet {
		return quietOut(responses)
	}

	if strings.ToLower(networkListOptions.Format) == "json" {
		return jsonOut(responses)
	}

	nlprs := make([]ListPrintReports, 0, len(responses))
	for _, r := range responses {
		nlprs = append(nlprs, ListPrintReports{r})
	}

	row := networkListOptions.Format
	if len(row) < 1 {
		row = defaultListRow
	}
	if !strings.HasSuffix(row, "\n") {
		row += "\n"
	}

	format := "{{range . }}" + row + "{{end}}"
	if !cmd.Flag("format").Changed {
		format = headers + format
	}
	tmpl, err := template.New("listNetworks").Parse(format)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 8, 2, 2, ' ', 0)
	if err := tmpl.Execute(w, nlprs); err != nil {
		return err
	}
	return w.Flush()
}

func quietOut(responses []*entities.NetworkListReport) error {
	for _, r := range responses {
		fmt.Println(r.Name)
	}
	return nil
}

func jsonOut(responses []*entities.NetworkListReport) error {
	b, err := json.MarshalIndent(responses, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

type ListPrintReports struct {
	*entities.NetworkListReport
}

func (n ListPrintReports) Version() string {
	return n.CNIVersion
}

func (n ListPrintReports) Plugins() string {
	return network.GetCNIPlugins(n.NetworkConfigList)
}
