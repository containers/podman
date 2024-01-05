//go:build amd64 || arm64

package machine

import (
	"fmt"
	"time"

	"github.com/containers/podman/v4/pkg/machine/p5"

	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/qemu"
	"github.com/containers/podman/v4/pkg/machine/vmconfigs"
	"github.com/sirupsen/logrus"
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

	// TODO this is for QEMU only (change to generic when adding second provider)
	q := new(qemu.QEMUStubber)
	dirs, err := machine.GetMachineDirs(q.VMType())
	if err != nil {
		return err
	}
	mc, err := vmconfigs.LoadMachineByName(vmName, dirs)
	if err != nil {
		return err
	}

	if err := p5.Stop(mc, q, dirs, false); err != nil {
		return err
	}

	// Update last time up
	mc.LastUp = time.Now()
	if err := mc.Write(); err != nil {
		logrus.Errorf("unable to write configuration file: %q", err)
	}

	fmt.Printf("Machine %q stopped successfully\n", vmName)
	newMachineEvent(events.Stop, events.Event{Name: vmName})
	return nil
}
