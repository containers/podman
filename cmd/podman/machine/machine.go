// +build amd64 arm64

package machine

import (
	"strings"

	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/spf13/cobra"
)

var (
	noOp = func(cmd *cobra.Command, args []string) error {
		return nil
	}
	// Command: podman _machine_
	machineCmd = &cobra.Command{
		Use:                "machine",
		Short:              "Manage a virtual machine",
		Long:               "Manage a virtual machine. Virtual machines are used to run Podman.",
		PersistentPreRunE:  noOp,
		PersistentPostRunE: noOp,
		RunE:               validate.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: machineCmd,
	})
}

// autocompleteMachineSSH - Autocomplete machine ssh command.
func autocompleteMachineSSH(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return getMachines(toComplete)
	}
	return nil, cobra.ShellCompDirectiveDefault
}

// autocompleteMachine - Autocomplete machines.
func autocompleteMachine(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return getMachines(toComplete)
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

func getMachines(toComplete string) ([]string, cobra.ShellCompDirective) {
	suggestions := []string{}
	provider := getSystemDefaultProvider()
	machines, err := provider.List(machine.ListOptions{})
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	for _, m := range machines {
		if strings.HasPrefix(m.Name, toComplete) {
			suggestions = append(suggestions, m.Name)
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}
