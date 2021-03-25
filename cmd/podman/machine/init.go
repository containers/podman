// +build amd64,linux amd64,darwin arm64,darwin

package machine

import (
	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/machine"
	"github.com/containers/podman/v3/pkg/machine/qemu"
	"github.com/spf13/cobra"
)

var (
	initCmd = &cobra.Command{
		Use:               "init [options] [NAME]",
		Short:             "initialize a vm",
		Long:              "initialize a virtual machine for Podman to run on. Virtual machines are used to run Podman.",
		RunE:              initMachine,
		Args:              cobra.MaximumNArgs(1),
		Example:           `podman machine init myvm`,
		ValidArgsFunction: completion.AutocompleteNone,
	}
)

type InitCLIOptions struct {
	CPUS         uint64
	Memory       uint64
	Devices      []string
	ImagePath    string
	IgnitionPath string
	Name         string
}

var (
	initOpts                  = InitCLIOptions{}
	defaultMachineName string = "podman-machine-default"
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
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
	initOpts.Name = defaultMachineName
	if len(args) > 0 {
		initOpts.Name = args[0]
	}
	vmOpts := machine.InitOptions{
		CPUS:         initOpts.CPUS,
		Memory:       initOpts.Memory,
		IgnitionPath: initOpts.IgnitionPath,
		ImagePath:    initOpts.ImagePath,
		Name:         initOpts.Name,
	}
	var (
		vm     machine.VM
		vmType string
		err    error
	)
	switch vmType {
	default: // qemu is the default
		vm, err = qemu.NewMachine(vmOpts)
	}
	if err != nil {
		return err
	}
	return vm.Init(vmOpts)
}
