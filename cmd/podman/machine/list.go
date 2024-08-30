//go:build amd64 || arm64

package machine

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/machine"
	provider2 "github.com/containers/podman/v5/pkg/machine/provider"
	"github.com/containers/podman/v5/pkg/machine/shim"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"github.com/docker/go-units"
	"github.com/spf13/cobra"
)

var (
	lsCmd = &cobra.Command{
		Use:     "list [options]",
		Aliases: []string{"ls"},
		Short:   "List machines",
		Long:    "List managed virtual machines.",
		// do not use machinePreRunE, as that pre-sets the provider
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
	format       string
	noHeading    bool
	quiet        bool
	allProviders bool
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
	flags.BoolVar(&listFlag.allProviders, "all-providers", false, "Show machines from all providers")
}

func list(cmd *cobra.Command, args []string) error {
	var (
		opts machine.ListOptions
		err  error
	)
	var providers []vmconfigs.VMProvider
	if listFlag.allProviders {
		providers = provider2.GetAll()
	} else {
		provider, err = provider2.Get()
		if err != nil {
			return err
		}
		providers = []vmconfigs.VMProvider{provider}
	}

	listResponse, err := shim.List(providers, opts)
	if err != nil {
		return err
	}

	// Sort by last run
	sort.Slice(listResponse, func(i, j int) bool {
		return listResponse[i].LastUp.After(listResponse[j].LastUp)
	})
	// Bring currently running machines to top
	sort.Slice(listResponse, func(i, j int) bool {
		return listResponse[i].Running
	})

	// ignore the error here we only want to know if we have a default connection to show it in list
	defaultCon, _ := registry.PodmanConfig().ContainersConfDefaultsRO.GetConnection("", true)

	if report.IsJSON(listFlag.format) {
		machineReporter := toMachineFormat(listResponse, defaultCon)
		b, err := json.MarshalIndent(machineReporter, "", "    ")
		if err != nil {
			return err
		}
		os.Stdout.Write(b)
		return nil
	}

	machineReporter := toHumanFormat(listResponse, defaultCon)
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

func toMachineFormat(vms []*machine.ListResponse, defaultCon *config.Connection) []*entities.ListReporter {
	machineResponses := make([]*entities.ListReporter, 0, len(vms))
	for _, vm := range vms {
		isDefault := false
		// check port, in case we somehow have machines with the same name in different providers
		if defaultCon != nil {
			isDefault = vm.Name == defaultCon.Name && strings.Contains(defaultCon.URI, strconv.Itoa((vm.Port)))
		}
		response := new(entities.ListReporter)
		response.Default = isDefault
		response.Name = vm.Name
		response.Running = vm.Running
		response.LastUp = strTime(vm.LastUp)
		response.Created = strTime(vm.CreatedAt)
		response.Stream = streamName(vm.Stream)
		response.VMType = vm.VMType
		response.CPUs = vm.CPUs
		response.Memory = strUint(uint64(vm.Memory.ToBytes()))
		response.DiskSize = strUint(uint64(vm.DiskSize.ToBytes()))
		response.Port = vm.Port
		response.RemoteUsername = vm.RemoteUsername
		response.IdentityPath = vm.IdentityPath
		response.Starting = vm.Starting
		response.UserModeNetworking = vm.UserModeNetworking

		machineResponses = append(machineResponses, response)
	}
	return machineResponses
}

func toHumanFormat(vms []*machine.ListResponse, defaultCon *config.Connection) []*entities.ListReporter {
	humanResponses := make([]*entities.ListReporter, 0, len(vms))
	for _, vm := range vms {
		response := new(entities.ListReporter)
		isDefault := false
		// check port, in case we somehow have machines with the same name in different providers
		if defaultCon != nil {
			isDefault = vm.Name == defaultCon.Name && strings.Contains(defaultCon.URI, strconv.Itoa((vm.Port)))
		}
		if isDefault {
			response.Name = vm.Name + "*"
			response.Default = true
		} else {
			response.Name = vm.Name
		}
		switch {
		case vm.Starting:
			response.LastUp = "Currently starting"
			response.Starting = true
		case vm.Running:
			response.LastUp = "Currently running"
			response.Running = true
		case vm.LastUp.IsZero():
			response.LastUp = "Never"
		default:
			response.LastUp = units.HumanDuration(time.Since(vm.LastUp)) + " ago"
		}
		response.Created = units.HumanDuration(time.Since(vm.CreatedAt)) + " ago"
		response.VMType = vm.VMType
		response.CPUs = vm.CPUs
		response.Memory = units.BytesSize(float64(vm.Memory.ToBytes()))
		response.DiskSize = units.BytesSize(float64(vm.DiskSize.ToBytes()))

		humanResponses = append(humanResponses, response)
	}
	return humanResponses
}
