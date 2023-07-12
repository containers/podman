//go:build amd64 || arm64
// +build amd64 arm64

package machine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/spf13/cobra"
)

var (
	startCmd = &cobra.Command{
		Use:               "start [options] [MACHINE]",
		Short:             "Start an existing machine",
		Long:              "Start a managed virtual machine ",
		PersistentPreRunE: rootlessOnly,
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
		vm  machine.VM
	)

	startOpts.NoInfo = startOpts.Quiet || startOpts.NoInfo

	vmName := defaultMachineName
	if len(args) > 0 && len(args[0]) > 0 {
		vmName = args[0]
	}

	provider, err := GetSystemProvider()
	if err != nil {
		return err
	}

	vm, err = provider.LoadVMByName(vmName)
	if err != nil {
		return err
	}

	active, activeName, cerr := provider.CheckExclusiveActiveVM()
	if cerr != nil {
		return cerr
	}
	if active {
		if vmName == activeName {
			return fmt.Errorf("cannot start VM %s: %w", vmName, machine.ErrVMAlreadyRunning)
		}
		return fmt.Errorf("cannot start VM %s. VM %s is currently running or starting: %w", vmName, activeName, machine.ErrMultipleActiveVM)
	}
	if !startOpts.Quiet {
		fmt.Printf("Starting machine %q\n", vmName)
	}
	if err := vm.Start(vmName, startOpts); err != nil {
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
