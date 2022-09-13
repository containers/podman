//go:build amd64 || arm64
// +build amd64 arm64

package machine

import (
	"fmt"
	"os"
	"runtime"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/report"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
)

var infoDescription = `Display information pertaining to the machine host.`

var (
	infoCmd = &cobra.Command{
		Use:               "info [options]",
		Short:             "Display machine host info",
		Long:              infoDescription,
		PersistentPreRunE: rootlessOnly,
		RunE:              info,
		Args:              validate.NoArgs,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman machine info`,
	}
)

var (
	inFormat string
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: infoCmd,
		Parent:  machineCmd,
	})

	flags := infoCmd.Flags()
	formatFlagName := "format"
	flags.StringVarP(&inFormat, formatFlagName, "f", "", "Change the output format to JSON or a Go template")
	_ = infoCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&entities.MachineInfo{}))
}

func info(cmd *cobra.Command, args []string) error {
	info := entities.MachineInfo{}
	version, err := define.GetVersion()
	if err != nil {
		return fmt.Errorf("getting version info %w", err)
	}
	info.Version = version

	host, err := hostInfo()
	if err != nil {
		return err
	}
	info.Host = host

	switch {
	case report.IsJSON(inFormat):
		b, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	case cmd.Flags().Changed("format"):
		rpt := report.New(os.Stdout, cmd.Name())
		defer rpt.Flush()

		// Use OriginUnknown so it does not add an extra range since it
		// will only be called for a single element and not a slice.
		rpt, err = rpt.Parse(report.OriginUnknown, inFormat)
		if err != nil {
			return err
		}
		return rpt.Execute(info)
	default:
		b, err := yaml.Marshal(info)
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	}

	return nil
}

func hostInfo() (*entities.MachineHostInfo, error) {
	host := entities.MachineHostInfo{}

	host.Arch = runtime.GOARCH
	host.OS = runtime.GOOS

	provider := GetSystemDefaultProvider()
	var listOpts machine.ListOptions
	listResponse, err := provider.List(listOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to get machines %w", err)
	}

	host.NumberOfMachines = len(listResponse)

	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return nil, err
	}

	// Default state of machine is stopped
	host.MachineState = "Stopped"
	for _, vm := range listResponse {
		// Set default machine if found
		if vm.Name == cfg.Engine.ActiveService {
			host.DefaultMachine = vm.Name
		}
		// If machine is running or starting, it is automatically the current machine
		if vm.Running {
			host.CurrentMachine = vm.Name
			host.MachineState = "Running"
		} else if vm.Starting {
			host.CurrentMachine = vm.Name
			host.MachineState = "Starting"
		}
	}
	// If no machines are starting or running, set current machine to default machine
	// If no default machines are found, do not report a default machine or a state
	if host.CurrentMachine == "" {
		if host.DefaultMachine == "" {
			host.MachineState = ""
		} else {
			host.CurrentMachine = host.DefaultMachine
		}
	}

	host.VMType = provider.VMType()

	dataDir, err := machine.GetDataDir(host.VMType)
	if err != nil {
		return nil, fmt.Errorf("failed to get machine image dir")
	}
	host.MachineImageDir = dataDir

	confDir, err := machine.GetConfDir(host.VMType)
	if err != nil {
		return nil, fmt.Errorf("failed to get machine config dir %w", err)
	}
	host.MachineConfigDir = confDir

	eventsDir, err := eventSockDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get events dir: %w", err)
	}
	host.EventsDir = eventsDir

	return &host, nil
}
