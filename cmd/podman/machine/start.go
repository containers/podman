package machine

import (
	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/machine"
	"github.com/containers/podman/v3/pkg/machine/qemu"
	"github.com/spf13/cobra"
)

var (
	startCmd = &cobra.Command{
		Use:               "start NAME",
		Short:             "Start an existing machine",
		Long:              "Start an existing machine ",
		RunE:              start,
		Args:              cobra.ExactArgs(1),
		Example:           `podman machine start myvm`,
		ValidArgsFunction: completion.AutocompleteNone,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: startCmd,
		Parent:  machineCmd,
	})
}

func start(cmd *cobra.Command, args []string) error {
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
	return vm.Start(args[0], machine.StartOptions{})
}
