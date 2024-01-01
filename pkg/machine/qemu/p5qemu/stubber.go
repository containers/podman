package p5qemu

import (
	"fmt"

	"github.com/containers/podman/v4/pkg/machine/define"
	"github.com/containers/podman/v4/pkg/machine/qemu/command"
	"github.com/containers/podman/v4/pkg/machine/vmconfigs"
	"github.com/go-openapi/errors"
)

type QEMUStubber struct {
	vmconfigs.QEMUConfig
}

func (q *QEMUStubber) CreateVM(opts define.CreateVMOpts, mc *vmconfigs.MachineConfig) error {
	fmt.Println("//// CreateVM: ", opts.Name)
	monitor, err := command.NewQMPMonitor(opts.Name, opts.Dirs.RuntimeDir)
	if err != nil {
		return err
	}
	qemuConfig := vmconfigs.QEMUConfig{
		Command:    nil,
		QMPMonitor: monitor,
	}

	mc.QEMUHypervisor = &qemuConfig
	return nil
}

func (q *QEMUStubber) StartVM() error {
	return errors.NotImplemented("")
}

func (q *QEMUStubber) StopVM() error {
	return errors.NotImplemented("")
}

func (q *QEMUStubber) InspectVM() error {
	return errors.NotImplemented("")
}

func (q *QEMUStubber) RemoveVM() error {
	return errors.NotImplemented("")
}

func (q *QEMUStubber) ChangeSettings() error {
	return errors.NotImplemented("")
}

func (q *QEMUStubber) IsFirstBoot() error {
	return errors.NotImplemented("")
}

func (q *QEMUStubber) SetupMounts() error {
	return errors.NotImplemented("")
}

func (q *QEMUStubber) CheckExclusiveActiveVM() (bool, string, error) {
	return false, "", errors.NotImplemented("")
}

func (q *QEMUStubber) GetHyperVisorVMs() ([]string, error) {
	return nil, nil
}

func (q *QEMUStubber) VMType() define.VMType {
	return define.QemuVirt
}
