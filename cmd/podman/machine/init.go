// +build amd64,linux arm64,linux amd64,darwin arm64,darwin

package machine

import (
	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/pkg/machine"
	"github.com/containers/podman/v3/pkg/machine/qemu"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	initCmd = &cobra.Command{
		Use:               "init [options] [NAME]",
		Short:             "Initialize a virtual machine",
		Long:              "initialize a virtual machine ",
		RunE:              initMachine,
		Args:              cobra.MaximumNArgs(1),
		Example:           `podman machine init myvm`,
		ValidArgsFunction: completion.AutocompleteNone,
	}
)

var (
	initOpts           = machine.InitOptions{}
	defaultMachineName = "podman-machine-default"
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: initCmd,
		Parent:  machineCmd,
	})
	flags := initCmd.Flags()

	cpusFlagName := "cpus"
	flags.Uint64Var(
		&initOpts.CPUS,
		cpusFlagName, 1,
		"Number of CPUs. The default is 1.",
	)
	_ = initCmd.RegisterFlagCompletionFunc(cpusFlagName, completion.AutocompleteNone)

	diskSizeFlagName := "disk-size"
	flags.Uint64Var(
		&initOpts.DiskSize,
		diskSizeFlagName, 10,
		"Disk size in GB",
	)

	_ = initCmd.RegisterFlagCompletionFunc(diskSizeFlagName, completion.AutocompleteNone)

	memoryFlagName := "memory"
	flags.Uint64VarP(
		&initOpts.Memory,
		memoryFlagName, "m", 2048,
		"Memory (in MB)",
	)
	_ = initCmd.RegisterFlagCompletionFunc(memoryFlagName, completion.AutocompleteNone)

	ImagePathFlagName := "image-path"
	flags.StringVar(&initOpts.ImagePath, ImagePathFlagName, "", "Path to qcow image")
	_ = initCmd.RegisterFlagCompletionFunc(ImagePathFlagName, completion.AutocompleteDefault)

	IgnitionPathFlagName := "ignition-path"
	flags.StringVar(&initOpts.IgnitionPath, IgnitionPathFlagName, "", "Path to ignition file")
	_ = initCmd.RegisterFlagCompletionFunc(IgnitionPathFlagName, completion.AutocompleteDefault)
}

// TODO should we allow for a users to append to the qemu cmdline?
func initMachine(cmd *cobra.Command, args []string) error {
	var (
		vm     machine.VM
		vmType string
		err    error
	)
	initOpts.Name = defaultMachineName
	if len(args) > 0 {
		initOpts.Name = args[0]
	}
	switch vmType {
	default: // qemu is the default
		if _, err := qemu.LoadVMByName(initOpts.Name); err == nil {
			return errors.Wrap(machine.ErrVMAlreadyExists, initOpts.Name)
		}
		vm, err = qemu.NewMachine(initOpts)
	}
	if err != nil {
		return err
	}
	return vm.Init(initOpts)
}
