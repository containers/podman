// +build amd64,linux arm64,linux amd64,darwin arm64,darwin

package machine

import (
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/machine"
	"github.com/containers/podman/v3/pkg/machine/qemu"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	sshCmd = &cobra.Command{
		Use:   "ssh [options] [MACHINE] [COMMAND [ARG ...]]",
		Short: "SSH into a virtual machine",
		Long:  "SSH into a virtual machine ",
		RunE:  ssh,
		Args:  cobra.MaximumNArgs(1),
		Example: `podman machine ssh myvm
  podman machine ssh -e  myvm echo hello`,

		ValidArgsFunction: autocompleteMachineSSH,
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
}

func ssh(cmd *cobra.Command, args []string) error {
	var (
		err    error
		vm     machine.VM
		vmType string
	)
	vmName := defaultMachineName
	if len(args) > 0 && len(args[0]) > 1 {
		vmName = args[0]
	}
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
		vm, err = qemu.LoadVMByName(vmName)
	}
	if err != nil {
		return errors.Wrapf(err, "vm %s not found", args[0])
	}
	return vm.SSH(vmName, sshOpts)
}
