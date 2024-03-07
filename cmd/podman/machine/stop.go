//go:build amd64 || arm64

package machine

import (
	"fmt"

	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/libpod/events"
	"github.com/containers/podman/v5/pkg/machine/env"
	"github.com/containers/podman/v5/pkg/machine/shim"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"github.com/spf13/cobra"
)

var (
	stopCmd = &cobra.Command{
		Use:               "stop [MACHINE]",
		Short:             "Stop an existing machine",
		Long:              "Stop a managed virtual machine ",
		PersistentPreRunE: machinePreRunE,
		RunE:              stop,
		Args:              cobra.MaximumNArgs(1),
		Example:           `podman machine stop podman-machine-default`,
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
	)

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

	if err := shim.Stop(mc, provider, dirs, false); err != nil {
		return err
	}

	fmt.Printf("Machine %q stopped successfully\n", vmName)
	newMachineEvent(events.Stop, events.Event{Name: vmName})
	return nil
}
