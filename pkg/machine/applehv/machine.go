//go:build arm64 && !windows && !linux
// +build arm64,!windows,!linux

package applehv

import (
	"time"

	"github.com/containers/podman/v4/pkg/machine"
)

type Provider struct{}

var (
	hvProvider = &Provider{}
	// vmtype refers to qemu (vs libvirt, krun, etc).
	vmtype = "apple"
)

func GetVirtualizationProvider() machine.Provider {
	return hvProvider
}

const (
	// Some of this will need to change when we are closer to having
	// working code.
	VolumeTypeVirtfs     = "virtfs"
	MountType9p          = "9p"
	dockerSock           = "/var/run/docker.sock"
	dockerConnectTimeout = 5 * time.Second
	apiUpTimeout         = 20 * time.Second
)

type apiForwardingState int

const (
	noForwarding apiForwardingState = iota
	claimUnsupported
	notInstalled
	machineLocal
	dockerGlobal
)

func (p *Provider) NewMachine(opts machine.InitOptions) (machine.VM, error) {
	return nil, machine.ErrNotImplemented
}

func (p *Provider) LoadVMByName(name string) (machine.VM, error) {
	return nil, machine.ErrNotImplemented
}

func (p *Provider) List(opts machine.ListOptions) ([]*machine.ListResponse, error) {
	return nil, machine.ErrNotImplemented
}

func (p *Provider) IsValidVMName(name string) (bool, error) {
	return false, machine.ErrNotImplemented
}

func (p *Provider) CheckExclusiveActiveVM() (bool, string, error) {
	return false, "", machine.ErrNotImplemented
}

func (p *Provider) RemoveAndCleanMachines() error {
	return machine.ErrNotImplemented
}

func (p *Provider) VMType() string {
	return vmtype
}
