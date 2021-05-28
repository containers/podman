// +build amd64,linux arm64,linux amd64,darwin arm64,darwin

package machine

import (
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/pkg/machine"
	"github.com/containers/podman/v3/pkg/machine/qemu"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	startCmd = &cobra.Command{
		Use:               "start [MACHINE]",
		Short:             "Start an existing machine",
		Long:              "Start a managed virtual machine ",
		RunE:              start,
		Args:              cobra.MaximumNArgs(1),
		Example:           `podman machine start myvm`,
		ValidArgsFunction: autocompleteMachine,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: startCmd,
		Parent:  machineCmd,
	})
}

func start(cmd *cobra.Command, args []string) error {
	var (
		err    error
		vm     machine.VM
		vmType string
	)
	vmName := defaultMachineName
	if len(args) > 0 && len(args[0]) > 0 {
		vmName = args[0]
	}

	// We only have qemu VM's for now
	active, activeName, err := qemu.CheckActiveVM()
	if err != nil {
		return err
	}
	if active {
		if vmName == activeName {
			return errors.Wrapf(machine.ErrVMAlreadyRunning, "cannot start VM %s", vmName)
		}
		return errors.Wrapf(machine.ErrMultipleActiveVM, "cannot start VM %s. VM %s is currently running", vmName, activeName)
	}
	switch vmType {
	default:
		vm, err = qemu.LoadVMByName(vmName)
	}
	if err != nil {
		return err
	}
	return vm.Start(vmName, machine.StartOptions{})
}
