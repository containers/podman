// +build amd64,linux arm64,linux amd64,darwin arm64,darwin

package machine

import (
	"errors"
	"runtime"
	"strings"

	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/validate"
	"github.com/containers/podman/v3/pkg/machine"
	"github.com/containers/podman/v3/pkg/machine/qemu"
	"github.com/spf13/cobra"
)

var (
	noOp = func(cmd *cobra.Command, args []string) error {
		return nil
	}
	// noAarch64 temporarily disables arm64 support on
	// Apple Silicon
	noAarch64 = func(cmd *cobra.Command, args []string) error {
		if runtime.GOARCH == "arm64" {
			if runtime.GOOS == "darwin" {
				return errors.New("due to missing upstream patches, Apple Silicon is not capable of running Podman machine yet")
			}
			return errors.New("no aarch64 images are available at this time")
		}
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
	machines, err := qemu.List(machine.ListOptions{})
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
