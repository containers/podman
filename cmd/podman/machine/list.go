//go:build amd64 || arm64
// +build amd64 arm64

package machine

import (
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/pkg/machine"
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
  podman machine list --format json
  podman machine ls`,
	}
	listFlag = listFlagType{}
)

type listFlagType struct {
	format    string
	noHeading bool
	quiet     bool
}

type ListReporter struct {
	Name           string
	Default        bool
	Created        string
	Running        bool
	Starting       bool
	LastUp         string
	Stream         string
	VMType         string
	CPUs           uint64
	Memory         string
	DiskSize       string
	Port           int
	RemoteUsername string
	IdentityPath   string
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: lsCmd,
		Parent:  machineCmd,
	})

	flags := lsCmd.Flags()
	formatFlagName := "format"
	flags.StringVar(&listFlag.format, formatFlagName, "{{.Name}}\t{{.VMType}}\t{{.Created}}\t{{.LastUp}}\t{{.CPUs}}\t{{.Memory}}\t{{.DiskSize}}\n", "Format volume output using JSON or a Go template")
	_ = lsCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&ListReporter{}))
	flags.BoolVar(&listFlag.noHeading, "noheading", false, "Do not print headers")
	flags.BoolVarP(&listFlag.quiet, "quiet", "q", false, "Show only machine names")
}

func list(cmd *cobra.Command, args []string) error {
	var (
		opts         machine.ListOptions
		listResponse []*machine.ListResponse
		err          error
	)

	if listFlag.quiet {
		listFlag.format = "{{.Name}}\n"
	}

	provider := GetSystemDefaultProvider()
	listResponse, err = provider.List(opts)
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

	if report.IsJSON(listFlag.format) {
		machineReporter, err := toMachineFormat(listResponse)
		if err != nil {
			return err
		}

		b, err := json.MarshalIndent(machineReporter, "", "    ")
		if err != nil {
			return err
		}
		os.Stdout.Write(b)
		return nil
	}

	machineReporter, err := toHumanFormat(listResponse)
	if err != nil {
		return err
	}

	return outputTemplate(cmd, machineReporter)
}

func outputTemplate(cmd *cobra.Command, responses []*ListReporter) error {
	headers := report.Headers(ListReporter{}, map[string]string{
		"LastUp":   "LAST UP",
		"VmType":   "VM TYPE",
		"CPUs":     "CPUS",
		"Memory":   "MEMORY",
		"DiskSize": "DISK SIZE",
	})
	printHeader := !listFlag.noHeading
	if listFlag.quiet {
		printHeader = false
	}
	var row string
	switch {
	case cmd.Flags().Changed("format"):
		row = cmd.Flag("format").Value.String()
		listFlag.noHeading = !report.HasTable(row)
		row = report.NormalizeFormat(row)
	default:
		row = cmd.Flag("format").Value.String()
	}
	format := report.EnforceRange(row)

	tmpl, err := report.NewTemplate("list").Parse(format)
	if err != nil {
		return err
	}

	w, err := report.NewWriterDefault(os.Stdout)
	if err != nil {
		return err
	}
	defer w.Flush()
	if printHeader {
		if err := tmpl.Execute(w, headers); err != nil {
			return errors.Wrapf(err, "failed to write report column headers")
		}
	}
	return tmpl.Execute(w, responses)
}

func strTime(t time.Time) string {
	iso, err := t.MarshalText()
	if err != nil {
		return ""
	}
	return string(iso)
}

func strUint(u uint64) string {
	return strconv.FormatUint(u, 10)
}

func streamName(imageStream string) string {
	if imageStream == "" {
		return "default"
	}
	return imageStream
}

func toMachineFormat(vms []*machine.ListResponse) ([]*ListReporter, error) {
	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return nil, err
	}

	machineResponses := make([]*ListReporter, 0, len(vms))
	for _, vm := range vms {
		response := new(ListReporter)
		response.Default = vm.Name == cfg.Engine.ActiveService
		response.Name = vm.Name
		response.Running = vm.Running
		response.LastUp = strTime(vm.LastUp)
		response.Created = strTime(vm.CreatedAt)
		response.Stream = streamName(vm.Stream)
		response.VMType = vm.VMType
		response.CPUs = vm.CPUs
		response.Memory = strUint(vm.Memory)
		response.DiskSize = strUint(vm.DiskSize)
		response.Port = vm.Port
		response.RemoteUsername = vm.RemoteUsername
		response.IdentityPath = vm.IdentityPath

		machineResponses = append(machineResponses, response)
	}
	return machineResponses, nil
}

func toHumanFormat(vms []*machine.ListResponse) ([]*ListReporter, error) {
	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return nil, err
	}

	humanResponses := make([]*ListReporter, 0, len(vms))
	for _, vm := range vms {
		response := new(ListReporter)
		if vm.Name == cfg.Engine.ActiveService {
			response.Name = vm.Name + "*"
			response.Default = true
		} else {
			response.Name = vm.Name
		}
		switch {
		case vm.Running:
			response.LastUp = "Currently running"
			response.Running = true
		case vm.Starting:
			response.LastUp = "Currently starting"
			response.Starting = true
		default:
			response.LastUp = units.HumanDuration(time.Since(vm.LastUp)) + " ago"
		}
		response.Created = units.HumanDuration(time.Since(vm.CreatedAt)) + " ago"
		response.VMType = vm.VMType
		response.CPUs = vm.CPUs
		response.Memory = units.HumanSize(float64(vm.Memory))
		response.DiskSize = units.HumanSize(float64(vm.DiskSize))

		humanResponses = append(humanResponses, response)
	}
	return humanResponses, nil
}
