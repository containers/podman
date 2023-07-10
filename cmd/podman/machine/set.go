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
		Short:             "Set a virtual machine setting",
		Long:              "Set an updatable virtual machine setting",
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
	CPUs               uint64
	DiskSize           uint64
	Memory             uint64
	Rootful            bool
	UserModeNetworking bool
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
		"Disk size in GiB",
	)

	_ = setCmd.RegisterFlagCompletionFunc(diskSizeFlagName, completion.AutocompleteNone)

	memoryFlagName := "memory"
	flags.Uint64VarP(
		&setFlags.Memory,
		memoryFlagName, "m", 0,
		"Memory in MiB",
	)
	_ = setCmd.RegisterFlagCompletionFunc(memoryFlagName, completion.AutocompleteNone)

	userModeNetFlagName := "user-mode-networking"
	flags.BoolVar(&setFlags.UserModeNetworking, userModeNetFlagName, false, // defaults not-relevant due to use of Changed()
		"Whether this machine should use user-mode networking, routing traffic through a host user-space process")
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
	provider, err := GetSystemProvider()
	if err != nil {
		return err
	}
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
	if cmd.Flags().Changed("user-mode-networking") {
		setOpts.UserModeNetworking = &setFlags.UserModeNetworking
	}

	setErrs, lasterr := vm.Set(vmName, setOpts)
	for _, err := range setErrs {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}

	return lasterr
}
