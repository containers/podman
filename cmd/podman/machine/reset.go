//go:build amd64 || arm64

package machine

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/containers/podman/v5/pkg/machine"
	provider2 "github.com/containers/podman/v5/pkg/machine/provider"
	"github.com/containers/podman/v5/pkg/machine/shim"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
)

var (
	resetCmd = &cobra.Command{
		Use:               "reset [options]",
		Short:             "Remove all machines",
		Long:              "Remove all machines, configurations, data, and cached images",
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
	allProviders := provider2.GetAll()
	for _, p := range allProviders {
		hasPerms := provider2.HasPermsForProvider(p.VMType())
		isInstalled, err := provider2.IsInstalled(p.VMType())
		if !hasPerms && (isInstalled || err != nil) && !resetOptions.Force {
			logrus.Warnf("Managing %s machines require admin authority.", p.VMType().String())
			logrus.Warnf("Continuing to reset may cause Podman to be unaware of remaining VMs in the VM manager.")
		}
	}

	if !resetOptions.Force {
		listResponse, err := shim.List(allProviders, machine.ListOptions{})
		if err != nil {
			return err
		}

		resetConfirmationMessage(listResponse)

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
	return shim.Reset(allProviders, resetOptions)
}

func resetConfirmationMessage(listResponse []*machine.ListResponse) {
	fmt.Println("Warning: this command will delete all existing Podman machines")
	fmt.Println("and all of the configuration and data directories for Podman machines")
	fmt.Printf("\nThe following machine(s) will be deleted:\n\n")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tPROVIDER")

	for _, m := range listResponse {
		fmt.Fprintf(w, "%s\t%s\n", m.Name, m.VMType)
	}
	w.Flush()
}
