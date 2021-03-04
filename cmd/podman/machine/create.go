package machine

import (
	"fmt"
	"strings"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	createCmd = &cobra.Command{
		Use:               "create [options] NAME",
		Short:             "Create a vm",
		Long:              "Create a virtual machine for Podman to run on. Virtual machines are used to run Podman on Macs. ",
		RunE:              create,
		Args:              cobra.ExactArgs(1),
		Example:           `podman machine create myvm`,
		ValidArgsFunction: completion.AutocompleteNone,
	}
)

type CreateCLIOptions struct {
	CPUS       uint64
	Memory     uint64
	KernelPath string
	Devices    []string
}

var (
	createOpts = CreateCLIOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: createCmd,
		Parent:  machineCmd,
	})
	flags := createCmd.Flags()

	cpusFlagName := "cpus"
	flags.Uint64Var(
		&createOpts.CPUS,
		cpusFlagName, 0,
		"Number of CPUs. The default is 0.000 which means no limit",
	)
	_ = createCmd.RegisterFlagCompletionFunc(cpusFlagName, completion.AutocompleteNone)

	memoryFlagName := "memory"
	flags.Uint64VarP(
		&createOpts.Memory,
		memoryFlagName, "m", 0,
		"Memory (in MB)",
	)
	_ = createCmd.RegisterFlagCompletionFunc(memoryFlagName, completion.AutocompleteNone)

	kernelPathFlagName := "kernel-path"
	flags.StringVar(
		&createOpts.KernelPath,
		kernelPathFlagName, "",
		"Kernel path",
	)
	_ = createCmd.RegisterFlagCompletionFunc(kernelPathFlagName, completion.AutocompleteNone)

	deviceFlagName := "device"
	flags.StringSliceVar(
		&createOpts.Devices,
		deviceFlagName, []string{},
		"Add a device",
	)
	_ = createCmd.RegisterFlagCompletionFunc(deviceFlagName, completion.AutocompleteDefault)
}

func create(cmd *cobra.Command, args []string) error {
	vmOpts := CreateOptions{
		CPUS:       createOpts.CPUS,
		Memory:     createOpts.Memory,
		KernelPath: createOpts.KernelPath,
	}

	if cmd.Flags().Changed("device") {
		devices, err := cmd.Flags().GetStringSlice("device")
		if err != nil {
			return err
		}
		vmOpts.Devices, err = parseDevices(devices)
		if err != nil {
			return err
		}
	}

	test := new(TestVM)
	test.Create(args[0], vmOpts)

	return nil
}

func parseDevices(devices []string) ([]VMDevices, error) {
	vmDevices := make([]VMDevices, 0, len(devices))

	for _, dev := range devices {
		split := strings.Split(dev, ":")

		if len(split) == 1 {
			vmDevices = append(vmDevices, VMDevices{
				Path:     split[0],
				ReadOnly: false,
			})
		} else if len(split) == 2 {
			var readonly bool

			switch split[1] {
			case "ro", "readonly":
				readonly = true
			default:
				return nil, errors.New(fmt.Sprintf("Invalid readonly value: %s", dev))
			}

			vmDevices = append(vmDevices, VMDevices{
				Path:     split[0],
				ReadOnly: readonly,
			})
		} else {
			return nil, errors.New(fmt.Sprintf("Invalid device format: %s", dev))
		}
	}
	return vmDevices, nil
}
