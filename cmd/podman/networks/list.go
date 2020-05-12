package network

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/cmd/podman/validate"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/network"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	networklistDescription = `List networks`
	networklistCommand     = &cobra.Command{
		Use:     "ls",
		Args:    validate.NoArgs,
		Short:   "network list",
		Long:    networklistDescription,
		RunE:    networkList,
		Example: `podman network list`,
		Annotations: map[string]string{
			registry.ParentNSRequired: "",
		},
	}
)

var (
	networkListOptions entities.NetworkListOptions
	headers            string = "NAME\tVERSION\tPLUGINS\n"
	defaultListRow     string = "{{.Name}}\t{{.Version}}\t{{.Plugins}}\n"
)

func networkListFlags(flags *pflag.FlagSet) {
	// TODO enable filters based on something
	//flags.StringSliceVarP(&networklistCommand.Filter, "filter", "f",  []string{}, "Pause all running containers")
	flags.StringVarP(&networkListOptions.Format, "format", "f", "", "Pretty-print containers to JSON or using a Go template")
	flags.BoolVarP(&networkListOptions.Quiet, "quiet", "q", false, "display only names")
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
	var (
		nlprs []NetworkListPrintReports
	)

	responses, err := registry.ContainerEngine().NetworkList(registry.Context(), networkListOptions)
	if err != nil {
		return err
	}

	// quiet means we only print the network names
	if networkListOptions.Quiet {
		return quietOut(responses)
	}

	if networkListOptions.Format == "json" {
		return jsonOut(responses)
	}

	for _, r := range responses {
		nlprs = append(nlprs, NetworkListPrintReports{r})
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

type NetworkListPrintReports struct {
	*entities.NetworkListReport
}

func (n NetworkListPrintReports) Version() string {
	return n.CNIVersion
}

func (n NetworkListPrintReports) Plugins() string {
	return network.GetCNIPlugins(n.NetworkConfigList)
}
