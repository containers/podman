//go:build amd64 || arm64

package machine

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/env"
	"github.com/containers/podman/v5/pkg/machine/shim"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"github.com/spf13/cobra"
)

var (
	resetCmd = &cobra.Command{
		Use:               "reset [options]",
		Short:             "Remove all machines",
		Long:              "Remove all machines, configurations, data, and cached images",
		PersistentPreRunE: machinePreRunE,
		RunE:              reset,
		Args:              validate.NoArgs,
		Example:           `podman machine reset`,
		ValidArgsFunction: completion.AutocompleteNone,
	}
)

var (
	resetOptions machine.ResetOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: resetCmd,
		Parent:  machineCmd,
	})

	flags := resetCmd.Flags()
	formatFlagName := "force"
	flags.BoolVarP(&resetOptions.Force, formatFlagName, "f", false, "Do not prompt before reset")
}

func reset(_ *cobra.Command, _ []string) error {
	var (
		err error
	)

	dirs, err := env.GetMachineDirs(provider.VMType())
	if err != nil {
		return err
	}

	// TODO we could consider saying we get a list of vms but can proceed
	// to just delete all local disk dirs, etc.  Maybe a --proceed?
	mcs, err := vmconfigs.LoadMachinesInDir(dirs)
	if err != nil {
		return err
	}

	if !resetOptions.Force {
		vms := vmNamesFromMcs(mcs)
		resetConfirmationMessage(vms)
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("\nAre you sure you want to continue? [y/N] ")
		answer, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.ToLower(answer)[0] != 'y' {
			return nil
		}
	}

	// resetErr can be nil or a multi-error
	return shim.Reset(dirs, provider, mcs)
}

func resetConfirmationMessage(vms []string) {
	fmt.Println("Warning: this command will delete all existing Podman machines")
	fmt.Println("and all of the configuration and data directories for Podman machines")
	fmt.Printf("\nThe following machine(s) will be deleted:\n\n")
	for _, msg := range vms {
		fmt.Printf("%s\n", msg)
	}
}

func vmNamesFromMcs(mcs map[string]*vmconfigs.MachineConfig) []string {
	keys := make([]string, 0, len(mcs))
	for k := range mcs {
		keys = append(keys, k)
	}
	return keys
}
