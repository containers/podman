//go:build amd64 || arm64

package machine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/libpod/events"
	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/env"
	"github.com/containers/podman/v5/pkg/machine/shim"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
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

	dirs, err := env.GetMachineDirs(provider.VMType())
	if err != nil {
		return err
	}
	mc, err := vmconfigs.LoadMachineByName(vmName, dirs)
	if err != nil {
		return err
	}

	if !startOpts.Quiet {
		fmt.Printf("Starting machine %q\n", vmName)
	}

	if err := shim.Start(mc, provider, dirs, startOpts); err != nil {
		return err
	}
	if err := changeDockerContext(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to change docker context to default. Error: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "Docker context has been set to default\n")
	}
	fmt.Printf("Machine %q started successfully\n", vmName)
	newMachineEvent(events.Start, events.Event{Name: vmName})
	return nil
}

func changeDockerContext() error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	pathList := strings.Split(os.Getenv("PATH"), ":")
	var dockerBinaryPath string
	for _, path := range pathList {
		dockerPath := filepath.Join(path, "docker")
		if info, err := os.Stat(dockerPath); err == nil && info.Mode().IsRegular() {
			dockerBinaryPath = dockerPath
			break
		}
	}
	if dockerBinaryPath == "" {
		return fmt.Errorf("docker binary not found")
	}

	cmd := exec.Command(dockerBinaryPath, "context", "use", "default")

	// Run the command
	_, err := cmd.CombinedOutput()
	return err
}
