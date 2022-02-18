//go:build amd64 || arm64
// +build amd64 arm64

package machine

import (
	"fmt"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/machine"
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
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: startCmd,
		Parent:  machineCmd,
	})

	ProviderTypeFlagName := "type"
	flags := startCmd.Flags()
	flags.StringVar(&providerType, ProviderTypeFlagName, "", "Type of VM provider")
	_ = startCmd.RegisterFlagCompletionFunc(ProviderTypeFlagName, completion.AutocompleteNone)
}

func start(cmd *cobra.Command, args []string) error {
	var (
		err      error
		vm       machine.VM
		provider machine.Provider
	)

	provider, err = getProvider(providerType)
	if err != nil {
		return err
	}

	vmName := provider.DefaultVMName()

	if len(args) > 0 && len(args[0]) > 0 {
		vmName = args[0]
	}

	_, err = provider.LoadVMByName(vmName)
	if err != nil {
		return err
	}

	active, activeName, cerr := provider.CheckExclusiveActiveVM()
	if cerr != nil {
		return cerr
	}
	if active {
		if vmName == activeName {
			return errors.Wrapf(machine.ErrVMAlreadyRunning, "cannot start VM %s", vmName)
		}
		return errors.Wrapf(machine.ErrMultipleActiveVM, "cannot start VM %s. VM %s is currently running", vmName, activeName)
	}
	vm, err = provider.LoadVMByName(vmName)
	if err != nil {
		return err
	}
	fmt.Printf("Starting machine %q\n", vmName)
	if err := vm.Start(vmName, machine.StartOptions{}); err != nil {
		return err
	}
	fmt.Printf("Machine %q started successfully\n", vmName)
	return nil
}
