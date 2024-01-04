//go:build amd64 || arm64

package os

import (
	"fmt"

	"github.com/containers/podman/v4/pkg/machine"
)

// MachineOS manages machine OS's from outside the machine.
type MachineOS struct {
	Args    []string
	VM      machine.VM
	VMName  string
	Restart bool
}

// Apply applies the image by sshing into the machine and running apply from inside the VM.
func (m *MachineOS) Apply(image string, opts ApplyOptions) error {
	sshOpts := machine.SSHOptions{
		Args: []string{"podman", "machine", "os", "apply", image},
	}

	if err := m.VM.SSH(m.VMName, sshOpts); err != nil {
		return err
	}

	if m.Restart {
		if err := m.VM.Stop(m.VMName, machine.StopOptions{}); err != nil {
			return err
		}
		if err := m.VM.Start(m.VMName, machine.StartOptions{NoInfo: true}); err != nil {
			return err
		}
		fmt.Printf("Machine %q restarted successfully\n", m.VMName)
	}
	return nil
}
