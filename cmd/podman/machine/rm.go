//go:build amd64 || arm64

package machine

import (
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/libpod/events"
	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/env"
	"github.com/containers/podman/v5/pkg/machine/shim"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"github.com/spf13/cobra"
)

var (
	rmCmd = &cobra.Command{
		Use:               "rm [options] [MACHINE]",
		Short:             "Remove an existing machine",
		Long:              "Remove a managed virtual machine ",
		PersistentPreRunE: machinePreRunE,
		RunE:              rm,
		Args:              cobra.MaximumNArgs(1),
		Example:           `podman machine rm podman-machine-default`,
		ValidArgsFunction: autocompleteMachine,
	}
)

var (
	destroyOptions machine.RemoveOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmCmd,
		Parent:  machineCmd,
	})

	flags := rmCmd.Flags()
	formatFlagName := "force"
	flags.BoolVarP(&destroyOptions.Force, formatFlagName, "f", false, "Stop and do not prompt before rming")

	ignitionFlagName := "save-ignition"
	flags.BoolVar(&destroyOptions.SaveIgnition, ignitionFlagName, false, "Do not delete ignition file")

	imageFlagName := "save-image"
	flags.BoolVar(&destroyOptions.SaveImage, imageFlagName, false, "Do not delete the image file")
}

func rm(_ *cobra.Command, args []string) error {
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

	if err := shim.Remove(mc, provider, dirs, destroyOptions); err != nil {
		return err
	}
	newMachineEvent(events.Remove, events.Event{Name: vmName})
	return nil
}
