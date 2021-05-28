// +build amd64,linux arm64,linux amd64,darwin arm64,darwin

package machine

import (
	"os"
	"sort"
	"text/tabwriter"
	"text/template"
	"time"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v3/cmd/podman/parse"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/validate"
	"github.com/containers/podman/v3/pkg/machine"
	"github.com/containers/podman/v3/pkg/machine/qemu"
	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	lsCmd = &cobra.Command{
		Use:               "list [options]",
		Aliases:           []string{"ls"},
		Short:             "List machines",
		Long:              "List managed virtual machines.",
		RunE:              list,
		Args:              validate.NoArgs,
		ValidArgsFunction: completion.AutocompleteNone,
		Example: `podman machine list,
  podman machine ls`,
	}
	listFlag = listFlagType{}
)

type listFlagType struct {
	format    string
	noHeading bool
}

type machineReporter struct {
	Name    string
	Created string
	LastUp  string
	VMType  string
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: lsCmd,
		Parent:  machineCmd,
	})

	flags := lsCmd.Flags()
	formatFlagName := "format"
	flags.StringVar(&listFlag.format, formatFlagName, "{{.Name}}\t{{.VMType}}\t{{.Created}}\t{{.LastUp}}\n", "Format volume output using Go template")
	_ = lsCmd.RegisterFlagCompletionFunc(formatFlagName, completion.AutocompleteNone)
	flags.BoolVar(&listFlag.noHeading, "noheading", false, "Do not print headers")
}

func list(cmd *cobra.Command, args []string) error {
	var opts machine.ListOptions
	// We only have qemu VM's for now
	listResponse, err := qemu.List(opts)
	if err != nil {
		return errors.Wrap(err, "error listing vms")
	}

	// Sort by last run
	sort.Slice(listResponse, func(i, j int) bool {
		return listResponse[i].LastUp.After(listResponse[j].LastUp)
	})
	// Bring currently running machines to top
	sort.Slice(listResponse, func(i, j int) bool {
		return listResponse[i].Running
	})
	machineReporter, err := toHumanFormat(listResponse)
	if err != nil {
		return err
	}

	return outputTemplate(cmd, machineReporter)
}

func outputTemplate(cmd *cobra.Command, responses []*machineReporter) error {
	headers := report.Headers(machineReporter{}, map[string]string{
		"LastUp": "LAST UP",
		"VmType": "VM TYPE",
	})

	row := report.NormalizeFormat(listFlag.format)
	format := parse.EnforceRange(row)

	tmpl, err := template.New("list machines").Parse(format)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 12, 2, 2, ' ', 0)
	defer w.Flush()

	if cmd.Flags().Changed("format") && !parse.HasTable(listFlag.format) {
		listFlag.noHeading = true
	}

	if !listFlag.noHeading {
		if err := tmpl.Execute(w, headers); err != nil {
			return errors.Wrapf(err, "failed to write report column headers")
		}
	}
	return tmpl.Execute(w, responses)
}

func toHumanFormat(vms []*machine.ListResponse) ([]*machineReporter, error) {
	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return nil, err
	}

	humanResponses := make([]*machineReporter, 0, len(vms))
	for _, vm := range vms {
		response := new(machineReporter)
		if vm.Name == cfg.Engine.ActiveService {
			response.Name = vm.Name + "*"
		} else {
			response.Name = vm.Name
		}
		if vm.Running {
			response.LastUp = "Currently running"
		} else {
			response.LastUp = units.HumanDuration(time.Since(vm.LastUp)) + " ago"
		}
		response.Created = units.HumanDuration(time.Since(vm.CreatedAt)) + " ago"
		response.VMType = vm.VMType

		humanResponses = append(humanResponses, response)
	}
	return humanResponses, nil
}
