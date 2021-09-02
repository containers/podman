// +build amd64,linux arm64,linux amd64,darwin arm64,darwin

package machine

import (
	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/pkg/machine"
	"github.com/containers/podman/v3/pkg/machine/qemu"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	mountCmd = &cobra.Command{
		Use:               "mount [NAME] [options]",
		Short:             "mount vm directory to host",
		Long:              "mount vm directory to host",
		RunE:              mount,
		Example:           `podman machine mount myvm --local=/foo --remote=/bar`,
		ValidArgsFunction: completion.AutocompleteNone,
	}
)

var (
	sshfsOptions machine.SSHFSOptions
)

func init() {
	mountCmd.Flags().SetInterspersed(false)
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: mountCmd,
		Parent:  machineCmd,
	})

	flags := rmCmd.Flags()
	localFlagName := "local"
	flags.StringVar(&sshfsOptions.Local, localFlagName, "", "local directory from host")

	remoteFlagName := "remote"
	flags.StringVar(&sshfsOptions.Remote, remoteFlagName, "", "remote directory in vm")
}

func mount(cmd *cobra.Command, args []string) error {
	var (
		err     error
		validVM bool
		vm      machine.VM
		vmType  string
	)

	// Set the VM to default
	vmName := defaultMachineName
	// If len is greater than 0, it means we may have been
	// provided the VM name.  If so, we check.  The VM name,
	// if provided, must be in args[0].
	if len(args) > 0 {
		switch vmType {
		default:
			validVM, err = qemu.IsValidVMName(args[0])
			if err != nil {
				return err
			}
			if validVM {
				vmName = args[0]
			} else {
				return errors.Errorf("%s is not a valid vm", args[0])
			}
		}
	}

	switch vmType {
	default:
		vm, err = qemu.LoadVMByName(vmName)
	}
	if err != nil {
		return errors.Wrapf(err, "vm %s not found", vmName)
	}
	return vm.Mount(vmName, sshfsOptions)
}
