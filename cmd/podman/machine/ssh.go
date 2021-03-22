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
		Use:   "ssh [options] NAME [COMMAND [ARG ...]]",
		Short: "SSH into a virtual machine",
		Long:  "SSH into a podman-managed virtual machine ",
		RunE:  ssh,
		Args:  cobra.MinimumNArgs(1),
		Example: `podman machine ssh myvm
  podman machine ssh -e  myvm echo hello`,

		ValidArgsFunction: completion.AutocompleteNone,
	}
)

var (
	sshOpts machine.SSHOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: sshCmd,
		Parent:  machineCmd,
	})

	flags := sshCmd.Flags()
	executeFlagName := "execute"
	flags.BoolVarP(&sshOpts.Execute, executeFlagName, "e", false, "Execute command from args")
	_ = sshCmd.RegisterFlagCompletionFunc(executeFlagName, completion.AutocompleteDefault)
}

func ssh(cmd *cobra.Command, args []string) error {
	var (
		err    error
		vm     machine.VM
		vmType string
	)
	sshOpts.Args = args[1:]

	// Error if no execute but args given
	if !sshOpts.Execute && len(sshOpts.Args) > 0 {
		return errors.New("too many args: to execute commands via ssh, use -e flag")
	}
	// Error if execute but no args given
	if sshOpts.Execute && len(sshOpts.Args) < 1 {
		return errors.New("must proivde at least one command to execute")
	}

	switch vmType {
	default:
		vm, err = qemu.LoadVMByName(args[0])
	}
	if err != nil {
		return errors.Wrapf(err, "vm %s not found", args[0])
	}
	return vm.SSH(args[0], sshOpts)
}
