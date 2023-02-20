//go:build arm64 && darwin
// +build arm64,darwin

package applehv

import "github.com/containers/podman/v4/pkg/machine"

type Virtualization struct {
	artifact    machine.Artifact
	compression machine.ImageCompression
	format      machine.ImageFormat
}

func (v Virtualization) Artifact() machine.Artifact {
	return machine.None
}

func (v Virtualization) CheckExclusiveActiveVM() (bool, string, error) {
	return false, "", machine.ErrNotImplemented
}

func (v Virtualization) Compression() machine.ImageCompression {
	return v.compression
}

func (v Virtualization) Format() machine.ImageFormat {
	return v.format
}

func (v Virtualization) IsValidVMName(name string) (bool, error) {
	return false, machine.ErrNotImplemented
}

func (v Virtualization) List(opts machine.ListOptions) ([]*machine.ListResponse, error) {
	return nil, machine.ErrNotImplemented
}

func (v Virtualization) LoadVMByName(name string) (machine.VM, error) {
	return nil, machine.ErrNotImplemented
}

func (v Virtualization) NewMachine(opts machine.InitOptions) (machine.VM, error) {
	return nil, machine.ErrNotImplemented
}

func (v Virtualization) RemoveAndCleanMachines() error {
	return machine.ErrNotImplemented
}

func (v Virtualization) VMType() string {
	return vmtype
}
