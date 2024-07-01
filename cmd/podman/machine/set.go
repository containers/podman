//go:build amd64 || arm64

package machine

import (
	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/strongunits"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/env"
	"github.com/containers/podman/v5/pkg/machine/shim"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"github.com/spf13/cobra"
)

var (
	setCmd = &cobra.Command{
		Use:               "set [options] [NAME]",
		Short:             "Set a virtual machine setting",
		Long:              "Set an updatable virtual machine setting",
		PersistentPreRunE: machinePreRunE,
		RunE:              setMachine,
		Args:              cobra.MaximumNArgs(1),
		Example:           `podman machine set --rootful=false`,
		ValidArgsFunction: completion.AutocompleteNone,
	}
)

var (
	setFlags = SetFlags{}
	setOpts  = define.SetOptions{}
)

type SetFlags struct {
	CPUs               uint64
	DiskSize           uint64
	Memory             uint64
	Rootful            bool
	UserModeNetworking bool
	USBs               []string
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

	usbFlagName := "usb"
	flags.StringArrayVarP(
		&setFlags.USBs,
		usbFlagName, "", []string{},
		"USBs bus=$1,devnum=$2 or vendor=$1,product=$2")
	_ = setCmd.RegisterFlagCompletionFunc(usbFlagName, completion.AutocompleteNone)

	userModeNetFlagName := "user-mode-networking"
	flags.BoolVar(&setFlags.UserModeNetworking, userModeNetFlagName, false, // defaults not-relevant due to use of Changed()
		"Whether this machine should use user-mode networking, routing traffic through a host user-space process")
}

func setMachine(cmd *cobra.Command, args []string) error {
	vmName := defaultMachineName
	if len(args) > 0 && len(args[0]) > 0 {
		vmName = args[0]
	}

	dirs, err := env.GetMachineDirs(provider.VMType())
	if err != nil {
		return err
	}

	mc, err := vmconfigs.LoadMachineByName(vmName, dirs)
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
		newMemory := strongunits.MiB(setFlags.Memory)
		if err := checkMaxMemory(newMemory); err != nil {
			return err
		}
		setOpts.Memory = &newMemory
	}
	if cmd.Flags().Changed("disk-size") {
		newDiskSizeGB := strongunits.GiB(setFlags.DiskSize)
		setOpts.DiskSize = &newDiskSizeGB
	}
	if cmd.Flags().Changed("user-mode-networking") {
		setOpts.UserModeNetworking = &setFlags.UserModeNetworking
	}
	if cmd.Flags().Changed("usb") {
		setOpts.USBs = &setFlags.USBs
	}

	// At this point, we have the known changed information, etc
	// Walk through changes to the providers if they need them
	return shim.Set(mc, provider, setOpts)
}
