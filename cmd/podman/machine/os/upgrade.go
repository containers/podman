//go:build amd64 || arm64

package os

import (
	"fmt"

	"github.com/blang/semver/v4"
	"github.com/containers/podman/v6/cmd/podman/common"
	"github.com/containers/podman/v6/cmd/podman/machine"
	"github.com/containers/podman/v6/cmd/podman/registry"
	"github.com/containers/podman/v6/cmd/podman/validate"
	machine2 "github.com/containers/podman/v6/pkg/machine"
	"github.com/containers/podman/v6/pkg/machine/os"
	"github.com/containers/podman/v6/pkg/machine/shim"
	"github.com/containers/podman/v6/version"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
)

var upgradeCmd = &cobra.Command{
	Use:               "upgrade [options] IMAGE [NAME]",
	Short:             "Upgrade machine os",
	Long:              "Upgrade the machine operating system to a newer version",
	PersistentPreRunE: validate.NoOp,
	Args:              cobra.MaximumNArgs(1),
	RunE:              upgrade,
	ValidArgsFunction: common.AutocompleteImages,
	Example:           `podman machine os upgrade`,
}

type upgradeOpts struct {
	dryRun      bool
	format      string
	hostVersion string
	restart     bool
}

var opts upgradeOpts

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: upgradeCmd,
		Parent:  machine.OSCmd,
	})
	flags := upgradeCmd.Flags()

	dryRunFlagName := "dry-run"
	flags.BoolVarP(&opts.dryRun, dryRunFlagName, "n", false, "Only check if an upgrade is available")
	hostVersionFlagName := "host-version"
	flags.StringVar(&opts.hostVersion, hostVersionFlagName, "", "Podman host version (advanced use only)")
	formatFlagName := "format"
	flags.StringVarP(&opts.format, formatFlagName, "f", "", "suppress output except for specified format. Implies -n")
	_ = upgradeCmd.RegisterFlagCompletionFunc(formatFlagName, completion.AutocompleteNone)
	restartFlagName := "restart"
	flags.BoolVar(&opts.restart, restartFlagName, false, "Restart after upgrade")
}

func upgrade(_ *cobra.Command, args []string) error {
	var vmName string
	if len(args) == 1 {
		vmName = args[0]
	}

	managerOpts := ManagerOpts{
		VMName:  vmName,
		CLIArgs: args,
		Restart: opts.restart,
	}

	osManager, err := NewOSManager(managerOpts)
	if err != nil {
		return err
	}

	upgradeOpts := os.UpgradeOptions{ClientVersion: version.Version, DryRun: opts.dryRun}
	if opts.hostVersion != "" {
		callerVersion, err := semver.ParseTolerant(opts.hostVersion)
		if err != nil {
			return err
		}
		upgradeOpts.MachineVersion = callerVersion
	}
	if kind := opts.format; len(opts.format) > 0 {
		// For now, we only support one output format value of `json`
		// But in the future, we may add additional formats as needed
		if kind != "json" {
			return errors.New("only value of `json` is supported")
		}
		upgradeOpts.Format = kind
	}

	if err := osManager.Upgrade(upgradeOpts); err != nil {
		return err
	}

	if opts.restart {
		fmt.Println("Restarting active machine after upgrade.")
		mc, vmProvider, err := shim.VMExists(vmName)
		if err != nil {
			return err
		}
		if err := shim.Stop(mc, vmProvider, false); err != nil {
			return err
		}
		var updateConn bool
		if err := shim.Start(mc, vmProvider, machine2.StartOptions{}, &updateConn); err != nil {
			return err
		}
	}
	return nil
}
