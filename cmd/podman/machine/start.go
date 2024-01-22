//go:build amd64 || arm64

package machine

import (
	"fmt"

	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/define"
	"github.com/containers/podman/v4/pkg/machine/shim"
	"github.com/containers/podman/v4/pkg/machine/vmconfigs"
	"github.com/sirupsen/logrus"
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
	startOpts = machine.StartOptions{}
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
}

func start(_ *cobra.Command, args []string) error {
	var (
		err error
	)

	startOpts.NoInfo = startOpts.Quiet || startOpts.NoInfo

	vmName := defaultMachineName
	if len(args) > 0 && len(args[0]) > 0 {
		vmName = args[0]
	}

	dirs, err := machine.GetMachineDirs(provider.VMType())
	if err != nil {
		return err
	}
	mc, err := vmconfigs.LoadMachineByName(vmName, dirs)
	if err != nil {
		return err
	}

	state, err := provider.State(mc, false)
	if err != nil {
		return err
	}

	if state == define.Running {
		return define.ErrVMAlreadyRunning
	}

	if err := shim.CheckExclusiveActiveVM(provider, mc); err != nil {
		return err
	}

	if !startOpts.Quiet {
		fmt.Printf("Starting machine %q\n", vmName)
	}

	// Set starting to true
	mc.Starting = true
	if err := mc.Write(); err != nil {
		logrus.Error(err)
	}

	// Set starting to false on exit
	defer func() {
		mc.Starting = false
		if err := mc.Write(); err != nil {
			logrus.Error(err)
		}
	}()
	if err := shim.Start(mc, provider, dirs, startOpts); err != nil {
		return err
	}
	fmt.Printf("Machine %q started successfully\n", vmName)
	newMachineEvent(events.Start, events.Event{Name: vmName})
	return nil
}
