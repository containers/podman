//go:build amd64 || arm64

package os

import (
	"bufio"
	"errors"
	"os"
	"strings"

	"github.com/containers/podman/v5/pkg/machine/define"
	pkgOS "github.com/containers/podman/v5/pkg/machine/os"
	"github.com/containers/podman/v5/pkg/machine/shim"
	machineconfig "go.podman.io/common/pkg/machine"
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

	// Set to the default name if no VM was provided
	if opts.VMName == "" {
		opts.VMName = define.DefaultMachineName
	}

	mc, vmProvider, err := shim.VMExists(opts.VMName)
	if err != nil {
		return nil, err
	}

	return &pkgOS.MachineOS{
		VM:       mc,
		Provider: vmProvider,
		Args:     opts.CLIArgs,
		VMName:   opts.VMName,
		Restart:  opts.Restart,
	}, nil
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
		if after, ok := strings.CutPrefix(l.Text(), "ID="); ok {
			dist.Name = after
		}
		if after, ok := strings.CutPrefix(l.Text(), "VARIANT_ID="); ok {
			dist.Variant = strings.Trim(after, "\"")
		}
	}
	return dist
}
