// +build amd64 arm64

package machine

import (
	"net/url"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	sshCmd = &cobra.Command{
		Use:   "ssh [options] [NAME] [COMMAND [ARG ...]]",
		Short: "SSH into an existing machine",
		Long:  "SSH into a managed virtual machine ",
		RunE:  ssh,
		Example: `podman machine ssh myvm
  podman machine ssh myvm echo hello`,
		ValidArgsFunction: autocompleteMachineSSH,
	}
)

var (
	sshOpts machine.SSHOptions
)

func init() {
	sshCmd.Flags().SetInterspersed(false)
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: sshCmd,
		Parent:  machineCmd,
	})
	flags := sshCmd.Flags()
	usernameFlagName := "username"
	flags.StringVar(&sshOpts.Username, usernameFlagName, "", "Username to use when ssh-ing into the VM.")
	_ = sshCmd.RegisterFlagCompletionFunc(usernameFlagName, completion.AutocompleteNone)
}

func ssh(cmd *cobra.Command, args []string) error {
	var (
		err     error
		validVM bool
		vm      machine.VM
	)

	// Set the VM to default
	vmName := defaultMachineName
	provider := getSystemDefaultProvider()

	// If len is greater than 0, it means we may have been
	// provided the VM name.  If so, we check.  The VM name,
	// if provided, must be in args[0].
	if len(args) > 0 {
		validVM, err = provider.IsValidVMName(args[0])
		if err != nil {
			return err
		}
		if validVM {
			vmName = args[0]
		} else {
			sshOpts.Args = append(sshOpts.Args, args[0])
		}
	}

	// If len is greater than 1, it means we might have been
	// given a vmname and args or just args
	if len(args) > 1 {
		if validVM {
			sshOpts.Args = args[1:]
		} else {
			sshOpts.Args = args
		}
	}

	if !validVM && sshOpts.Username == "" {
		sshOpts.Username, err = remoteConnectionUsername()
		if err != nil {
			return err
		}
	}

	vm, err = provider.LoadVMByName(vmName)
	if err != nil {
		return errors.Wrapf(err, "vm %s not found", vmName)
	}
	return vm.SSH(vmName, sshOpts)
}

func remoteConnectionUsername() (string, error) {
	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return "", err
	}
	dest, _, err := cfg.ActiveDestination()
	if err != nil {
		return "", err
	}
	uri, err := url.Parse(dest)
	if err != nil {
		return "", err
	}
	username := uri.User.String()
	return username, nil
}
