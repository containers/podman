package machine

import (
	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/machine"
	"github.com/containers/podman/v3/pkg/machine/qemu"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	sshCmd = &cobra.Command{
		Use:               "ssh NAME",
		Short:             "SSH into a virtual machine",
		Long:              "SSH into a podman-managed virtual machine ",
		RunE:              ssh,
		Args:              cobra.ExactArgs(1),
		Example:           `podman machine ssh myvm`,
		ValidArgsFunction: completion.AutocompleteNone,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: sshCmd,
		Parent:  machineCmd,
	})
}

func ssh(cmd *cobra.Command, args []string) error {
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
		return errors.Wrapf(err, "vm %s not found", args[0])
	}
	return vm.SSH(args[0], machine.SSHOptions{})
}
