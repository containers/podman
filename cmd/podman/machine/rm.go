//go:build amd64 || arm64
// +build amd64 arm64

package machine

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/spf13/cobra"
)

var (
	rmCmd = &cobra.Command{
		Use:               "rm [options] [MACHINE]",
		Short:             "Remove an existing machine",
		Long:              "Remove a managed virtual machine ",
		PersistentPreRunE: rootlessOnly,
		RunE:              rm,
		Args:              cobra.MaximumNArgs(1),
		Example:           `podman machine rm myvm`,
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

	keysFlagName := "save-keys"
	flags.BoolVar(&destroyOptions.SaveKeys, keysFlagName, false, "Do not delete SSH keys")

	ignitionFlagName := "save-ignition"
	flags.BoolVar(&destroyOptions.SaveIgnition, ignitionFlagName, false, "Do not delete ignition file")

	imageFlagName := "save-image"
	flags.BoolVar(&destroyOptions.SaveImage, imageFlagName, false, "Do not delete the image file")

	diskFlagName := "save-disks"
	flags.BoolVar(&destroyOptions.SaveDisks, diskFlagName, false, "Do not delete the disk file(s)")
}

func rm(_ *cobra.Command, args []string) error {
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
	confirmationMessage, remove, err := vm.Remove(vmName, destroyOptions)
	if err != nil {
		return err
	}

	if !destroyOptions.Force {
		// Warn user
		fmt.Println(confirmationMessage)
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Are you sure you want to continue? [y/N] ")
		answer, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.ToLower(answer)[0] != 'y' {
			return nil
		}
	}
	err = remove()
	if err != nil {
		return err
	}
	newMachineEvent(events.Remove, events.Event{Name: vmName})
	err = updateDefaultMachineInConfig(vmName)
	if err != nil {
		return fmt.Errorf("failed to update default machine: %v", err)
	}
	return nil
}

func updateDefaultMachineInConfig(vmName string) error {
	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return err
	}
	if cfg.Engine.ActiveService == vmName {
		cfg.Engine.ActiveService = ""
		for machine := range cfg.Engine.ServiceDestinations {
			cfg.Engine.ActiveService = machine
			break
		}
	}
	return cfg.Write()
}
