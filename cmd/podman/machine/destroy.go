package machine

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/machine"
	"github.com/containers/podman/v3/pkg/machine/qemu"
	"github.com/spf13/cobra"
)

var (
	destroyCmd = &cobra.Command{
		Use:               "destroy [options] NAME",
		Short:             "Destroy an existing machine",
		Long:              "Destroy an existing machine ",
		RunE:              destroy,
		Args:              cobra.ExactArgs(1),
		Example:           `podman machine destroy myvm`,
		ValidArgsFunction: completion.AutocompleteNone,
	}
)

var (
	destoryOptions machine.DestroyOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: destroyCmd,
		Parent:  machineCmd,
	})

	flags := destroyCmd.Flags()
	formatFlagName := "force"
	flags.BoolVar(&destoryOptions.Force, formatFlagName, false, "Do not prompt before destroying")
	_ = destroyCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteJSONFormat)

	keysFlagName := "save-keys"
	flags.BoolVar(&destoryOptions.SaveKeys, keysFlagName, false, "Do not delete SSH keys")
	_ = destroyCmd.RegisterFlagCompletionFunc(keysFlagName, common.AutocompleteJSONFormat)

	ignitionFlagName := "save-ignition"
	flags.BoolVar(&destoryOptions.SaveIgnition, ignitionFlagName, false, "Do not delete ignition file")
	_ = destroyCmd.RegisterFlagCompletionFunc(ignitionFlagName, common.AutocompleteJSONFormat)

	imageFlagName := "save-image"
	flags.BoolVar(&destoryOptions.SaveImage, imageFlagName, false, "Do not delete the image file")
	_ = destroyCmd.RegisterFlagCompletionFunc(imageFlagName, common.AutocompleteJSONFormat)
}

func destroy(cmd *cobra.Command, args []string) error {
	var (
		err    error
		vm     machine.VM
		vmType string
	)
	switch vmType {
	default:
		vm, err = qemu.LoadVMByName(args[0])
	}
	if err != nil {
		return err
	}
	confirmationMessage, doIt, err := vm.Destroy(args[0], machine.DestroyOptions{})
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
	return doIt()
}
