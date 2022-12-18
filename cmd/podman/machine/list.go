//go:build amd64 || arm64
// +build amd64 arm64

package machine

import (
	"fmt"
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
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/docker/go-units"
	"github.com/spf13/cobra"
)

var (
	lsCmd = &cobra.Command{
		Use:               "list [options]",
		Aliases:           []string{"ls"},
		Short:             "List machines",
		Long:              "List managed virtual machines.",
		PersistentPreRunE: rootlessOnly,
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

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: lsCmd,
		Parent:  machineCmd,
	})

	flags := lsCmd.Flags()
	formatFlagName := "format"
	flags.StringVar(&listFlag.format, formatFlagName, "{{range .}}{{.Name}}\t{{.VMType}}\t{{.Created}}\t{{.LastUp}}\t{{.CPUs}}\t{{.Memory}}\t{{.DiskSize}}\n{{end -}}", "Format volume output using JSON or a Go template")
	_ = lsCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&entities.ListReporter{}))
	flags.BoolVarP(&listFlag.noHeading, "noheading", "n", false, "Do not print headers")
	flags.BoolVarP(&listFlag.quiet, "quiet", "q", false, "Show only machine names")
}

func list(cmd *cobra.Command, args []string) error {
	var (
		opts         machine.ListOptions
		listResponse []*machine.ListResponse
		err          error
	)

	provider := GetSystemDefaultProvider()
	listResponse, err = provider.List(opts)
	if err != nil {
		return fmt.Errorf("listing vms: %w", err)
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

func outputTemplate(cmd *cobra.Command, responses []*entities.ListReporter) error {
	headers := report.Headers(entities.ListReporter{}, map[string]string{
		"LastUp":   "LAST UP",
		"VmType":   "VM TYPE",
		"CPUs":     "CPUS",
		"Memory":   "MEMORY",
		"DiskSize": "DISK SIZE",
	})

	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()

	var err error
	switch {
	case cmd.Flag("format").Changed:
		rpt, err = rpt.Parse(report.OriginUser, listFlag.format)
	case listFlag.quiet:
		rpt, err = rpt.Parse(report.OriginUser, "{{.Name}}\n")
	default:
		rpt, err = rpt.Parse(report.OriginPodman, listFlag.format)
	}
	if err != nil {
		return err
	}

	if rpt.RenderHeaders && !listFlag.noHeading {
		if err := rpt.Execute(headers); err != nil {
			return fmt.Errorf("failed to write report column headers: %w", err)
		}
	}
	return rpt.Execute(responses)
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

func toMachineFormat(vms []*machine.ListResponse) ([]*entities.ListReporter, error) {
	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return nil, err
	}

	machineResponses := make([]*entities.ListReporter, 0, len(vms))
	for _, vm := range vms {
		response := new(entities.ListReporter)
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
		response.Starting = vm.Starting

		machineResponses = append(machineResponses, response)
	}
	return machineResponses, nil
}

func toHumanFormat(vms []*machine.ListResponse) ([]*entities.ListReporter, error) {
	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return nil, err
	}

	humanResponses := make([]*entities.ListReporter, 0, len(vms))
	for _, vm := range vms {
		response := new(entities.ListReporter)
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
