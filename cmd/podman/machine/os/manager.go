//go:build amd64 || arm64

package os

import (
	"bufio"
	"errors"
	"os"
	"strings"

	machineconfig "github.com/containers/common/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/env"
	pkgOS "github.com/containers/podman/v5/pkg/machine/os"
	"github.com/containers/podman/v5/pkg/machine/provider"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
)

type ManagerOpts struct {
	VMName  string
	CLIArgs []string
	Restart bool
}

// NewOSManager creates a new OSManager depending on the mode of the call
func NewOSManager(opts ManagerOpts, p vmconfigs.VMProvider) (pkgOS.Manager, error) {
	// If a VM name is specified, then we know that we are not inside a
	// Podman VM, but rather outside of it.
	if machineconfig.IsPodmanMachine() && opts.VMName == "" {
		return guestOSManager()
	}
	return machineOSManager(opts, p)
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
func machineOSManager(opts ManagerOpts, _ vmconfigs.VMProvider) (pkgOS.Manager, error) {
	vmName := opts.VMName
	if opts.VMName == "" {
		vmName = define.DefaultMachineName
	}
	p, err := provider.Get()
	if err != nil {
		return nil, err
	}
	dirs, err := env.GetMachineDirs(p.VMType())
	if err != nil {
		return nil, err
	}
	mc, err := vmconfigs.LoadMachineByName(vmName, dirs)
	if err != nil {
		return nil, err
	}
	return &pkgOS.MachineOS{
		VM:       mc,
		Provider: p,
		Args:     opts.CLIArgs,
		VMName:   vmName,
		Restart:  opts.Restart,
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
