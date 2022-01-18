// +build amd64 arm64

package machine

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/spf13/cobra"
)

var (
	rmCmd = &cobra.Command{
		Use:               "rm [options] [MACHINE]",
		Short:             "Remove an existing machine",
		Long:              "Remove a managed virtual machine ",
		RunE:              rm,
		Args:              cobra.MaximumNArgs(1),
		Example:           `podman machine rm myvm`,
		ValidArgsFunction: autocompleteMachine,
	}
)

var (
	destoryOptions machine.RemoveOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmCmd,
		Parent:  machineCmd,
	})

	flags := rmCmd.Flags()
	formatFlagName := "force"
	flags.BoolVar(&destoryOptions.Force, formatFlagName, false, "Do not prompt before rming")

	keysFlagName := "save-keys"
	flags.BoolVar(&destoryOptions.SaveKeys, keysFlagName, false, "Do not delete SSH keys")

	ignitionFlagName := "save-ignition"
	flags.BoolVar(&destoryOptions.SaveIgnition, ignitionFlagName, false, "Do not delete ignition file")

	imageFlagName := "save-image"
	flags.BoolVar(&destoryOptions.SaveImage, imageFlagName, false, "Do not delete the image file")
}

func rm(cmd *cobra.Command, args []string) error {
	var (
		err error
		vm  machine.VM
	)
	vmName := defaultMachineName
	if len(args) > 0 && len(args[0]) > 0 {
		vmName = args[0]
	}

	provider := getSystemDefaultProvider()
	vm, err = provider.LoadVMByName(vmName)
	if err != nil {
		return err
	}
	confirmationMessage, remove, err := vm.Remove(vmName, machine.RemoveOptions{})
	if err != nil {
		return err
	}

	if !destoryOptions.Force {
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
	return remove()
}
