// +build amd64,!windows arm64,!windows

package machine

import (
	"encoding/json"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v3/cmd/podman/common"
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
	Name     string
	Default  bool
	Created  string
	Running  bool
	LastUp   string
	Stream   string
	VMType   string
	CPUs     uint64
	Memory   string
	DiskSize string
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: lsCmd,
		Parent:  machineCmd,
	})

	flags := lsCmd.Flags()
	formatFlagName := "format"
	flags.StringVar(&listFlag.format, formatFlagName, "{{.Name}}\t{{.VMType}}\t{{.Created}}\t{{.LastUp}}\t{{.CPUs}}\t{{.Memory}}\t{{.DiskSize}}\n", "Format volume output using JSON or a Go template")
	_ = lsCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(machineReporter{}))
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

	if report.IsJSON(listFlag.format) {
		machineReporter, err := toMachineFormat(listResponse)
		if err != nil {
			return err
		}

		b, err := json.Marshal(machineReporter)
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

func outputTemplate(cmd *cobra.Command, responses []*machineReporter) error {
	headers := report.Headers(machineReporter{}, map[string]string{
		"LastUp":   "LAST UP",
		"VmType":   "VM TYPE",
		"CPUs":     "CPUS",
		"Memory":   "MEMORY",
		"DiskSize": "DISK SIZE",
	})

	row := report.NormalizeFormat(listFlag.format)
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

	if cmd.Flags().Changed("format") && !report.HasTable(listFlag.format) {
		listFlag.noHeading = true
	}

	if !listFlag.noHeading {
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

func toMachineFormat(vms []*machine.ListResponse) ([]*machineReporter, error) {
	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return nil, err
	}

	machineResponses := make([]*machineReporter, 0, len(vms))
	for _, vm := range vms {
		response := new(machineReporter)
		response.Default = vm.Name == cfg.Engine.ActiveService
		response.Name = vm.Name
		response.Running = vm.Running
		response.LastUp = strTime(vm.LastUp)
		response.Created = strTime(vm.CreatedAt)
		response.Stream = streamName(vm.Stream)
		response.VMType = vm.VMType
		response.CPUs = vm.CPUs
		response.Memory = strUint(vm.Memory * units.MiB)
		response.DiskSize = strUint(vm.DiskSize * units.GiB)

		machineResponses = append(machineResponses, response)
	}
	return machineResponses, nil
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
			response.Default = true
		} else {
			response.Name = vm.Name
		}
		if vm.Running {
			response.LastUp = "Currently running"
			response.Running = true
		} else {
			response.LastUp = units.HumanDuration(time.Since(vm.LastUp)) + " ago"
		}
		response.Created = units.HumanDuration(time.Since(vm.CreatedAt)) + " ago"
		response.VMType = vm.VMType
		response.CPUs = vm.CPUs
		response.Memory = units.HumanSize(float64(vm.Memory) * units.MiB)
		response.DiskSize = units.HumanSize(float64(vm.DiskSize) * units.GiB)

		humanResponses = append(humanResponses, response)
	}
	return humanResponses, nil
}
