//go:build amd64 || arm64

package machine

import (
	"fmt"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/qemu"
	"github.com/containers/podman/v4/pkg/machine/vmconfigs"
	"github.com/containers/podman/v4/pkg/strongunits"
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
	setOpts  = machine.SetOptions{}
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
	var (
		err                error
		newCPUs, newMemory *uint64
		newDiskSize        *strongunits.GiB
	)

	vmName := defaultMachineName
	if len(args) > 0 && len(args[0]) > 0 {
		vmName = args[0]
	}

	provider := new(qemu.QEMUStubber)
	dirs, err := machine.GetMachineDirs(provider.VMType())
	if err != nil {
		return err
	}

	mc, err := vmconfigs.LoadMachineByName(vmName, dirs)
	if err != nil {
		return err
	}

	if cmd.Flags().Changed("rootful") {
		mc.HostUser.Rootful = setFlags.Rootful
	}
	if cmd.Flags().Changed("cpus") {
		mc.Resources.CPUs = setFlags.CPUs
		newCPUs = &mc.Resources.CPUs
	}
	if cmd.Flags().Changed("memory") {
		mc.Resources.Memory = setFlags.Memory
		newMemory = &mc.Resources.Memory
	}
	if cmd.Flags().Changed("disk-size") {
		if setFlags.DiskSize <= mc.Resources.DiskSize {
			return fmt.Errorf("new disk size must be larger than %d GB", mc.Resources.DiskSize)
		}
		mc.Resources.DiskSize = setFlags.DiskSize
		newDiskSizeGB := strongunits.GiB(setFlags.DiskSize)
		newDiskSize = &newDiskSizeGB
	}
	if cmd.Flags().Changed("user-mode-networking") {
		// TODO This needs help
		setOpts.UserModeNetworking = &setFlags.UserModeNetworking
	}
	if cmd.Flags().Changed("usb") {
		// TODO This needs help
		setOpts.USBs = &setFlags.USBs
	}

	// At this point, we have the known changed information, etc
	// Walk through changes to the providers if they need them
	if err := provider.SetProviderAttrs(mc, newCPUs, newMemory, newDiskSize); err != nil {
		return err
	}

	// Update the configuration file last if everything earlier worked
	return mc.Write()
}
