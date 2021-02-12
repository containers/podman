package network

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v2/cmd/podman/common"
	"github.com/containers/podman/v2/cmd/podman/parse"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/validate"
	"github.com/containers/podman/v2/libpod/network"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	networklistDescription = `List networks`
	networklistCommand     = &cobra.Command{
		Use:               "ls [options]",
		Args:              validate.NoArgs,
		Short:             "network list",
		Long:              networklistDescription,
		RunE:              networkList,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman network list`,
	}
)

var (
	networkListOptions entities.NetworkListOptions
	filters            []string
	noTrunc            bool
)

func networkListFlags(flags *pflag.FlagSet) {
	formatFlagName := "format"
	flags.StringVar(&networkListOptions.Format, formatFlagName, "", "Pretty-print networks to JSON or using a Go template")
	_ = networklistCommand.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteJSONFormat)

	flags.BoolVarP(&networkListOptions.Quiet, "quiet", "q", false, "display only names")
	flags.BoolVar(&noTrunc, "no-trunc", false, "Do not truncate the network ID")

	filterFlagName := "filter"
	flags.StringArrayVarP(&filters, filterFlagName, "f", nil, "Provide filter values (e.g. 'name=podman')")
	_ = networklistCommand.RegisterFlagCompletionFunc(filterFlagName, common.AutocompleteNetworkFilters)
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
	networkListOptions.Filters = make(map[string][]string)
	for _, f := range filters {
		split := strings.SplitN(f, "=", 2)
		if len(split) == 1 {
			return errors.Errorf("invalid filter %q", f)
		}
		networkListOptions.Filters[split[0]] = append(networkListOptions.Filters[split[0]], split[1])
	}
	responses, err := registry.ContainerEngine().NetworkList(registry.Context(), networkListOptions)
	if err != nil {
		return err
	}

	switch {
	case report.IsJSON(networkListOptions.Format):
		return jsonOut(responses)
	case networkListOptions.Quiet:
		// quiet means we only print the network names
		return quietOut(responses)
	}

	nlprs := make([]ListPrintReports, 0, len(responses))
	for _, r := range responses {
		nlprs = append(nlprs, ListPrintReports{r})
	}

	// Headers() gets lost resolving the embedded field names so add them
	headers := report.Headers(ListPrintReports{}, map[string]string{
		"Name":       "name",
		"CNIVersion": "version",
		"Version":    "version",
		"Plugins":    "plugins",
		"Labels":     "labels",
		"ID":         "network id",
	})
	renderHeaders := true
	row := "{{.Name}}\t{{.Version}}\t{{.Plugins}}\n"
	if cmd.Flags().Changed("format") {
		renderHeaders = parse.HasTable(networkListOptions.Format)
		row = report.NormalizeFormat(networkListOptions.Format)
	}
	format := parse.EnforceRange(row)

	tmpl, err := template.New("listNetworks").Parse(format)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 8, 2, 2, ' ', 0)
	defer w.Flush()

	if renderHeaders {
		if err := tmpl.Execute(w, headers); err != nil {
			return err
		}
	}
	return tmpl.Execute(w, nlprs)
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

func (n ListPrintReports) Labels() string {
	list := make([]string, 0, len(n.NetworkListReport.Labels))
	for k, v := range n.NetworkListReport.Labels {
		list = append(list, k+"="+v)
	}
	return strings.Join(list, ",")
}

func (n ListPrintReports) ID() string {
	length := 12
	if noTrunc {
		length = 64
	}
	return network.GetNetworkID(n.Name)[:length]
}
