// +build amd64,!windows arm64,!windows

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
		Use:               "start [options] [MACHINE]",
		Short:             "Start an existing machine",
		Long:              "Start a managed virtual machine ",
		RunE:              start,
		Args:              cobra.MaximumNArgs(1),
		Example:           `podman machine start myvm`,
		ValidArgsFunction: autocompleteMachine,
	}

	startOptions machine.StartOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: startCmd,
		Parent:  machineCmd,
	})
	flags := startCmd.Flags()
	flags.BoolVar(&startOptions.NoCompat, "nocompat", false, "Disable Docker API compatibility socket proxies")
	flags.BoolVar(&startOptions.NoLink, "nolink", false, "Do not link /var/run/docker.sock",)
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
	if err := vm.Start(vmName, startOptions); err != nil {
		return err
	}
	return nil
}
