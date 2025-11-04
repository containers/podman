//go:build amd64 || arm64

package machine

import (
	"fmt"

	"github.com/containers/podman/v6/cmd/podman/registry"
	"github.com/containers/podman/v6/libpod/events"
	"github.com/containers/podman/v6/pkg/machine"
	"github.com/containers/podman/v6/pkg/machine/shim"
	"github.com/spf13/cobra"
)

var (
	startCmd = &cobra.Command{
		Use:               "start [options] [MACHINE]",
		Short:             "Start an existing machine",
		Long:              "Start a managed virtual machine ",
		PersistentPreRunE: machinePreRunE,
		RunE:              start,
		Args:              cobra.MaximumNArgs(1),
		Example:           `podman machine start podman-machine-default`,
		ValidArgsFunction: autocompleteMachine,
	}
	startOpts            = machine.StartOptions{}
	setDefaultSystemConn bool
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: startCmd,
		Parent:  machineCmd,
	})

	flags := startCmd.Flags()
	noInfoFlagName := "no-info"
	flags.BoolVar(&startOpts.NoInfo, noInfoFlagName, false, "Suppress informational tips")

	quietFlagName := "quiet"
	flags.BoolVarP(&startOpts.Quiet, quietFlagName, "q", false, "Suppress machine starting status output")

	setDefaultConnectionFlagName := "update-connection"
	flags.BoolVarP(&setDefaultSystemConn, setDefaultConnectionFlagName, "u", false, "Set default system connection for this machine")
}

func start(cmd *cobra.Command, args []string) error {
	startOpts.NoInfo = startOpts.Quiet || startOpts.NoInfo
	vmName := defaultMachineName
	if len(args) > 0 && len(args[0]) > 0 {
		vmName = args[0]
	}

	mc, vmProvider, err := shim.VMExists(vmName)
	if err != nil {
		return err
	}

	if !startOpts.Quiet {
		fmt.Printf("Starting machine %q\n", vmName)
	}

	shouldUpdate := processSystemConnUpdate(cmd, setDefaultSystemConn)
	if err := shim.Start(mc, vmProvider, startOpts, shouldUpdate); err != nil {
		return err
	}
	fmt.Printf("Machine %q started successfully\n", vmName)
	newMachineEvent(events.Start, events.Event{Name: vmName})
	return nil
}

func processSystemConnUpdate(cmd *cobra.Command, updateVal bool) *bool {
	if !cmd.Flags().Changed("update-connection") {
		return nil
	}
	return &updateVal
}
