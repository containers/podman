//go:build amd64 || arm64
// +build amd64 arm64

package os

import (
	"bufio"
	"errors"
	"os"
	"strings"

	machineconfig "github.com/containers/common/pkg/machine"
	"github.com/containers/podman/v4/cmd/podman/machine"
	pkgMachine "github.com/containers/podman/v4/pkg/machine"
	pkgOS "github.com/containers/podman/v4/pkg/machine/os"
)

type ManagerOpts struct {
	VMName  string
	CLIArgs []string
	Restart bool
}

// NewOSManager creates a new OSManager depending on the mode of the call
func NewOSManager(opts ManagerOpts) (pkgOS.Manager, error) {
	// If a VM name is specified, then we know that we are not inside a
	// Podman VM, but rather outside of it.
	if machineconfig.IsPodmanMachine() && opts.VMName == "" {
		return guestOSManager()
	}
	return machineOSManager(opts)
}

// guestOSManager returns an OSmanager for inside-VM operations
func guestOSManager() (pkgOS.Manager, error) {
	dist := GetDistribution()
	switch {
	case dist.Name == "fedora" && dist.Variant == "coreos":
		return &pkgOS.OSTree{}, nil
	default:
		return nil, errors.New("unsupported OS")
	}
}

// machineOSManager returns an os manager that manages outside the VM.
func machineOSManager(opts ManagerOpts) (pkgOS.Manager, error) {
	vmName := opts.VMName
	if opts.VMName == "" {
		vmName = pkgMachine.DefaultMachineName
	}
	provider, err := machine.GetSystemProvider()
	if err != nil {
		return nil, err
	}
	vm, err := provider.LoadVMByName(vmName)
	if err != nil {
		return nil, err
	}
	return &pkgOS.MachineOS{
		VM:      vm,
		Args:    opts.CLIArgs,
		VMName:  vmName,
		Restart: opts.Restart,
	}, nil
}

type Distribution struct {
	Name    string
	Variant string
}

// GetDistribution checks the OS distribution
func GetDistribution() Distribution {
	dist := Distribution{
		Name:    "unknown",
		Variant: "unknown",
	}
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return dist
	}
	defer f.Close()

	l := bufio.NewScanner(f)
	for l.Scan() {
		if strings.HasPrefix(l.Text(), "ID=") {
			dist.Name = strings.TrimPrefix(l.Text(), "ID=")
		}
		if strings.HasPrefix(l.Text(), "VARIANT_ID=") {
			dist.Variant = strings.Trim(strings.TrimPrefix(l.Text(), "VARIANT_ID="), "\"")
		}
	}
	return dist
}
