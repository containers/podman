//go:build amd64 || arm64

package machine

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/define"
	"github.com/containers/podman/v4/pkg/machine/p5"
	"github.com/containers/podman/v4/pkg/machine/qemu"
	"github.com/containers/podman/v4/pkg/machine/vmconfigs"
	"github.com/sirupsen/logrus"
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

	state, err := q.State(mc, false)
	if err != nil {
		return err
	}

	if state == define.Running {
		if !destroyOptions.Force {
			return &define.ErrVMRunningCannotDestroyed{Name: vmName}
		}
		if err := p5.Stop(mc, q, dirs, true); err != nil {
			return err
		}
	}

	rmFiles, genericRm, err := mc.Remove(destroyOptions.SaveIgnition, destroyOptions.SaveImage)
	if err != nil {
		return err
	}

	providerFiles, providerRm, err := q.Remove(mc)
	if err != nil {
		return err
	}

	// Add provider specific files to the list
	rmFiles = append(rmFiles, providerFiles...)

	// Important!
	// Nothing can be removed at this point.  The user can still opt out below
	//

	if !destroyOptions.Force {
		// Warn user
		confirmationMessage(rmFiles)
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

	//
	// All actual removal of files and vms should occur after this
	//

	// TODO Should this be a hard error?
	if err := providerRm(); err != nil {
		logrus.Errorf("failed to remove virtual machine from provider for %q", vmName)
	}

	// TODO Should this be a hard error?
	if err := genericRm(); err != nil {
		logrus.Error("failed to remove machines files")
	}
	newMachineEvent(events.Remove, events.Event{Name: vmName})
	return nil
}

func confirmationMessage(files []string) {
	fmt.Printf("The following files will be deleted:\n\n\n")
	for _, msg := range files {
		fmt.Println(msg)
	}
}
