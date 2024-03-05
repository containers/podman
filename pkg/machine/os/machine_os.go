//go:build amd64 || arm64

package os

import (
	"fmt"

	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/env"
	"github.com/containers/podman/v5/pkg/machine/shim"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
)

// MachineOS manages machine OS's from outside the machine.
type MachineOS struct {
	Args     []string
	VM       *vmconfigs.MachineConfig
	Provider vmconfigs.VMProvider
	VMName   string
	Restart  bool
}

// Apply applies the image by sshing into the machine and running apply from inside the VM.
func (m *MachineOS) Apply(image string, opts ApplyOptions) error {
	args := []string{"podman", "machine", "os", "apply", image}

	if err := machine.CommonSSH(m.VM.SSH.RemoteUsername, m.VM.SSH.IdentityPath, m.VMName, m.VM.SSH.Port, args); err != nil {
		return err
	}

	dirs, err := env.GetMachineDirs(m.Provider.VMType())
	if err != nil {
		return err
	}

	if m.Restart {
		if err := shim.Stop(m.VM, m.Provider, dirs, false); err != nil {
			return err
		}
		if err := shim.Start(m.VM, m.Provider, dirs, machine.StartOptions{NoInfo: true}); err != nil {
			return err
		}
		fmt.Printf("Machine %q restarted successfully\n", m.VMName)
	}
	return nil
}
