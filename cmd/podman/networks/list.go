package network

import (
	"fmt"
	"os"
	"strings"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/validate"
	"github.com/containers/podman/v3/libpod/network/types"
	"github.com/containers/podman/v3/pkg/domain/entities"
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
	_ = networklistCommand.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(ListPrintReports{}))

	flags.BoolVarP(&networkListOptions.Quiet, "quiet", "q", false, "display only names")
	flags.BoolVar(&noTrunc, "no-trunc", false, "Do not truncate the network ID")

	filterFlagName := "filter"
	flags.StringArrayVarP(&filters, filterFlagName, "f", nil, "Provide filter values (e.g. 'name=podman')")
	flags.Bool("noheading", false, "Do not print headers")
	_ = networklistCommand.RegisterFlagCompletionFunc(filterFlagName, common.AutocompleteNetworkFilters)
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
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
	// quiet means we only print the network names
	case networkListOptions.Quiet:
		quietOut(responses)

	// JSON output formatting
	case report.IsJSON(networkListOptions.Format):
		err = jsonOut(responses)

	// table or other format output
	default:
		err = templateOut(responses, cmd)
	}

	return err
}

func quietOut(responses []types.Network) {
	for _, r := range responses {
		fmt.Println(r.Name)
	}
}

func jsonOut(responses []types.Network) error {
	prettyJSON, err := json.MarshalIndent(responses, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(prettyJSON))
	return nil
}

func templateOut(responses []types.Network, cmd *cobra.Command) error {
	nlprs := make([]ListPrintReports, 0, len(responses))
	for _, r := range responses {
		nlprs = append(nlprs, ListPrintReports{r})
	}

	// Headers() gets lost resolving the embedded field names so add them
	headers := report.Headers(ListPrintReports{}, map[string]string{
		"Name":   "name",
		"Driver": "driver",
		"Labels": "labels",
		"ID":     "network id",
	})

	renderHeaders := report.HasTable(networkListOptions.Format)
	var row, format string
	if cmd.Flags().Changed("format") {
		row = report.NormalizeFormat(networkListOptions.Format)
	} else { // 'podman network ls' equivalent to 'podman network ls --format="table {{.ID}} {{.Name}} {{.Version}} {{.Plugins}}" '
		renderHeaders = true
		row = "{{.ID}}\t{{.Name}}\t{{.Driver}}\n"
	}
	format = report.EnforceRange(row)

	tmpl, err := report.NewTemplate("list").Parse(format)
	if err != nil {
		return err
	}

	w, err := report.NewWriterDefault(os.Stdout)
	if err != nil {
		return err
	}
	defer w.Flush()

	noHeading, _ := cmd.Flags().GetBool("noheading")
	if !noHeading && renderHeaders {
		if err := tmpl.Execute(w, headers); err != nil {
			return err
		}
	}
	return tmpl.Execute(w, nlprs)
}

// ListPrintReports returns the network list report
type ListPrintReports struct {
	types.Network
}

// Labels returns any labels added to a Network
func (n ListPrintReports) Labels() string {
	list := make([]string, 0, len(n.Network.Labels))
	for k, v := range n.Network.Labels {
		list = append(list, k+"="+v)
	}
	return strings.Join(list, ",")
}

// ID returns the Podman Network ID
func (n ListPrintReports) ID() string {
	length := 12
	if noTrunc {
		length = 64
	}
	return n.Network.ID[:length]
}
