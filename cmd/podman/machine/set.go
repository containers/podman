//go:build amd64 || arm64
// +build amd64 arm64

package machine

import (
	"fmt"
	"os"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/spf13/cobra"
)

var (
	setCmd = &cobra.Command{
		Use:               "set [options] [NAME]",
		Short:             "Sets a virtual machine setting",
		Long:              "Sets an updatable virtual machine setting",
		PersistentPreRunE: rootlessOnly,
		RunE:              setMachine,
		Args:              cobra.MaximumNArgs(1),
		Example:           `podman machine set --rootful=false`,
		ValidArgsFunction: completion.AutocompleteNone,
	}
)

var (
	setFlags = SetFlags{}
	setOpts  = machine.SetOptions{}
)

type SetFlags struct {
	CPUs     uint64
	DiskSize uint64
	Memory   uint64
	Rootful  bool
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: setCmd,
		Parent:  machineCmd,
	})
	flags := setCmd.Flags()

	rootfulFlagName := "rootful"
	flags.BoolVar(&setFlags.Rootful, rootfulFlagName, false, "Whether this machine should prefer rootful container execution")

	cpusFlagName := "cpus"
	flags.Uint64Var(
		&setFlags.CPUs,
		cpusFlagName, 0,
		"Number of CPUs",
	)
	_ = setCmd.RegisterFlagCompletionFunc(cpusFlagName, completion.AutocompleteNone)

	diskSizeFlagName := "disk-size"
	flags.Uint64Var(
		&setFlags.DiskSize,
		diskSizeFlagName, 0,
		"Disk size in GB",
	)

	_ = setCmd.RegisterFlagCompletionFunc(diskSizeFlagName, completion.AutocompleteNone)

	memoryFlagName := "memory"
	flags.Uint64VarP(
		&setFlags.Memory,
		memoryFlagName, "m", 0,
		"Memory in MB",
	)
	_ = setCmd.RegisterFlagCompletionFunc(memoryFlagName, completion.AutocompleteNone)
}

func setMachine(cmd *cobra.Command, args []string) error {
	var (
		vm  machine.VM
		err error
	)

	vmName := defaultMachineName
	if len(args) > 0 && len(args[0]) > 0 {
		vmName = args[0]
	}
	provider := GetSystemDefaultProvider()
	vm, err = provider.LoadVMByName(vmName)
	if err != nil {
		return err
	}

	if cmd.Flags().Changed("rootful") {
		setOpts.Rootful = &setFlags.Rootful
	}
	if cmd.Flags().Changed("cpus") {
		setOpts.CPUs = &setFlags.CPUs
	}
	if cmd.Flags().Changed("memory") {
		setOpts.Memory = &setFlags.Memory
	}
	if cmd.Flags().Changed("disk-size") {
		setOpts.DiskSize = &setFlags.DiskSize
	}

	setErrs, lasterr := vm.Set(vmName, setOpts)
	for _, err := range setErrs {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}

	return lasterr
}
