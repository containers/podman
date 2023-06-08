package network

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/parse"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	networklistDescription = `List networks`
	networklistCommand     = &cobra.Command{
		Use:               "ls [options]",
		Args:              validate.NoArgs,
		Short:             "List networks",
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
	_ = networklistCommand.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&ListPrintReports{}))

	flags.BoolVarP(&networkListOptions.Quiet, "quiet", "q", false, "display only names")
	flags.BoolVar(&noTrunc, "no-trunc", false, "Do not truncate the network ID")

	filterFlagName := "filter"
	flags.StringArrayVarP(&filters, filterFlagName, "f", nil, "Provide filter values (e.g. 'name=podman')")
	flags.BoolP("noheading", "n", false, "Do not print headers")
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
	var err error
	networkListOptions.Filters, err = parse.FilterArgumentsIntoFilters(filters)
	if err != nil {
		return err
	}

	responses, err := registry.ContainerEngine().NetworkList(registry.Context(), networkListOptions)
	if err != nil {
		return err
	}
	// sort the networks to make sure the order is deterministic
	sort.Slice(responses, func(i, j int) bool {
		return responses[i].Name < responses[j].Name
	})

	switch {
	// quiet means we only print the network names
	case networkListOptions.Quiet:
		quietOut(responses)

	// JSON output formatting
	case report.IsJSON(networkListOptions.Format):
		err = jsonOut(responses)

	// table or other format output
	default:
		err = templateOut(cmd, responses)
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

func templateOut(cmd *cobra.Command, responses []types.Network) error {
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

	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()

	var err error
	switch {
	case cmd.Flag("format").Changed:
		rpt, err = rpt.Parse(report.OriginUser, networkListOptions.Format)
	default:
		rpt, err = rpt.Parse(report.OriginPodman, "{{range .}}{{.ID}}\t{{.Name}}\t{{.Driver}}\n{{end -}}")
	}
	if err != nil {
		return err
	}

	noHeading, _ := cmd.Flags().GetBool("noheading")
	if rpt.RenderHeaders && !noHeading {
		if err := rpt.Execute(headers); err != nil {
			return fmt.Errorf("failed to write report column headers: %w", err)
		}
	}
	return rpt.Execute(nlprs)
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
