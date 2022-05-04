//go:build amd64 || arm64
// +build amd64 arm64

package machine

import (
	"fmt"

	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/spf13/cobra"
)

var (
	stopCmd = &cobra.Command{
		Use:               "stop [MACHINE]",
		Short:             "Stop an existing machine",
		Long:              "Stop a managed virtual machine ",
		RunE:              stop,
		Args:              cobra.MaximumNArgs(1),
		Example:           `podman machine stop myvm`,
		ValidArgsFunction: autocompleteMachine,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: stopCmd,
		Parent:  machineCmd,
	})
}

// TODO  Name shouldn't be required, need to create a default vm
func stop(cmd *cobra.Command, args []string) error {
	var (
		err error
		vm  machine.VM
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
	if err := vm.Stop(vmName, machine.StopOptions{}); err != nil {
		return err
	}
	fmt.Printf("Machine %q stopped successfully\n", vmName)
	newMachineEvent(events.Stop, events.Event{Name: vmName})
	return nil
}
