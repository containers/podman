//go:build amd64 || arm64

package os

import (
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/cmd/podman/machine"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/cmd/podman/validate"
	"github.com/containers/podman/v5/pkg/machine/os"
	provider2 "github.com/containers/podman/v5/pkg/machine/provider"
	"github.com/spf13/cobra"
)

var (
	applyCmd = &cobra.Command{
		Use:               "apply [options] IMAGE [NAME]",
		Short:             "Apply an OCI image to a Podman Machine's OS",
		Long:              "Apply custom layers from a containerized Fedora CoreOS OCI image on top of an existing VM",
		PersistentPreRunE: validate.NoOp,
		Args:              cobra.RangeArgs(1, 2),
		RunE:              apply,
		ValidArgsFunction: common.AutocompleteImages,
		Example:           `podman machine os apply myimage`,
	}
)

var restart bool

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: applyCmd,
		Parent:  machine.OSCmd,
	})
	flags := applyCmd.Flags()

	restartFlagName := "restart"
	flags.BoolVar(&restart, restartFlagName, false, "Restart VM to apply changes")
}

func apply(cmd *cobra.Command, args []string) error {
	vmName := ""
	if len(args) == 2 {
		vmName = args[1]
	}
	managerOpts := ManagerOpts{
		VMName:  vmName,
		CLIArgs: args,
		Restart: restart,
	}

	provider, err := provider2.Get()
	if err != nil {
		return err
	}
	osManager, err := NewOSManager(managerOpts, provider)
	if err != nil {
		return err
	}

	applyOpts := os.ApplyOptions{
		Image: args[0],
	}
	return osManager.Apply(args[0], applyOpts)
}
