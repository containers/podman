//go:build amd64 || arm64

package machine

import (
	"fmt"
	"net/url"

	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/env"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/utils"
	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"github.com/spf13/cobra"
)

var (
	sshCmd = &cobra.Command{
		Use:               "ssh [options] [NAME] [COMMAND [ARG ...]]",
		Short:             "SSH into an existing machine",
		Long:              "SSH into a managed virtual machine ",
		PersistentPreRunE: machinePreRunE,
		RunE:              ssh,
		Example: `podman machine ssh podman-machine-default
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

// TODO Remember that this changed upstream and needs to updated as such!

func ssh(cmd *cobra.Command, args []string) error {
	var (
		err     error
		mc      *vmconfigs.MachineConfig
		validVM bool
	)

	dirs, err := env.GetMachineDirs(provider.VMType())
	if err != nil {
		return err
	}

	// Set the VM to default
	vmName := defaultMachineName
	// If len is greater than 0, it means we may have been
	// provided the VM name.  If so, we check.  The VM name,
	// if provided, must be in args[0].
	if len(args) > 0 {
		// note: previous incantations of this up by a specific name
		// and errors were ignored.  this error is not ignored because
		// it implies podman cannot read its machine files, which is bad
		machines, err := vmconfigs.LoadMachinesInDir(dirs)
		if err != nil {
			return err
		}

		mc, validVM = machines[args[0]]
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

	// If the machine config was not loaded earlier, we load it now
	if mc == nil {
		mc, err = vmconfigs.LoadMachineByName(vmName, dirs)
		if err != nil {
			return fmt.Errorf("vm %s not found: %w", vmName, err)
		}
	}

	if !validVM && sshOpts.Username == "" {
		sshOpts.Username, err = remoteConnectionUsername()
		if err != nil {
			return err
		}
	}

	state, err := provider.State(mc, false)
	if err != nil {
		return err
	}
	if state != define.Running {
		return fmt.Errorf("vm %q is not running", mc.Name)
	}

	username := sshOpts.Username
	if username == "" {
		username = mc.SSH.RemoteUsername
	}

	err = machine.CommonSSHShell(username, mc.SSH.IdentityPath, mc.Name, mc.SSH.Port, sshOpts.Args)
	return utils.HandleOSExecError(err)
}

func remoteConnectionUsername() (string, error) {
	con, err := registry.PodmanConfig().ContainersConfDefaultsRO.GetConnection("", true)
	if err != nil {
		return "", err
	}

	uri, err := url.Parse(con.URI)
	if err != nil {
		return "", err
	}
	username := uri.User.String()
	return username, nil
}
