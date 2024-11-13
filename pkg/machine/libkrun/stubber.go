//go:build darwin

package libkrun

import (
	"fmt"
	"strconv"

	gvproxy "github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/apple"
	"github.com/containers/podman/v5/pkg/machine/apple/vfkit"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/ignition"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"github.com/containers/podman/v5/utils"
	vfConfig "github.com/crc-org/vfkit/pkg/config"
)

const (
	krunkitBinary = "krunkit"
	localhostURI  = "http://localhost"
)

type LibKrunStubber struct {
	vmconfigs.AppleHVConfig
}

func (l *LibKrunStubber) CreateVM(opts define.CreateVMOpts, mc *vmconfigs.MachineConfig, builder *ignition.IgnitionBuilder) error {
	mc.LibKrunHypervisor = new(vmconfigs.LibKrunConfig)
	mc.LibKrunHypervisor.KRun = vfkit.Helper{}

	bl := vfConfig.NewEFIBootloader(fmt.Sprintf("%s/efi-bl-%s", opts.Dirs.DataDir.GetPath(), opts.Name), true)
	mc.LibKrunHypervisor.KRun.VirtualMachine = vfConfig.NewVirtualMachine(uint(mc.Resources.CPUs), uint64(mc.Resources.Memory), bl)

	randPort, err := utils.GetRandomPort()
	if err != nil {
		return err
	}
	mc.LibKrunHypervisor.KRun.Endpoint = localhostURI + ":" + strconv.Itoa(randPort)

	virtiofsMounts := make([]machine.VirtIoFs, 0, len(mc.Mounts))
	for _, mnt := range mc.Mounts {
		virtiofsMounts = append(virtiofsMounts, machine.MountToVirtIOFs(mnt))
	}

	// Populate the ignition file with virtiofs stuff
	virtIOIgnitionMounts, err := apple.GenerateSystemDFilesForVirtiofsMounts(virtiofsMounts)
	if err != nil {
		return err
	}
	builder.WithUnit(virtIOIgnitionMounts...)

	return apple.ResizeDisk(mc, mc.Resources.DiskSize)
}

func (l *LibKrunStubber) PrepareIgnition(mc *vmconfigs.MachineConfig, ignBuilder *ignition.IgnitionBuilder) (*ignition.ReadyUnitOpts, error) {
	return nil, nil
}

func (l *LibKrunStubber) Exists(name string) (bool, error) {
	// not applicable for libkrun (same as applehv)
	return false, nil
}

func (l *LibKrunStubber) MountType() vmconfigs.VolumeMountType {
	return vmconfigs.VirtIOFS
}

func (l *LibKrunStubber) MountVolumesToVM(mc *vmconfigs.MachineConfig, quiet bool) error {
	return nil
}

func (l *LibKrunStubber) Remove(mc *vmconfigs.MachineConfig) ([]string, func() error, error) {
	return []string{}, func() error { return nil }, nil
}

func (l *LibKrunStubber) RemoveAndCleanMachines(dirs *define.MachineDirs) error {
	return nil
}

func (l *LibKrunStubber) SetProviderAttrs(mc *vmconfigs.MachineConfig, opts define.SetOptions) error {
	state, err := l.State(mc, false)
	if err != nil {
		return err
	}
	return apple.SetProviderAttrs(mc, opts, state)
}

func (l *LibKrunStubber) StartNetworking(mc *vmconfigs.MachineConfig, cmd *gvproxy.GvproxyCommand) error {
	return apple.StartGenericNetworking(mc, cmd)
}

func (l *LibKrunStubber) PostStartNetworking(mc *vmconfigs.MachineConfig, noInfo bool) error {
	return nil
}

func (l *LibKrunStubber) StartVM(mc *vmconfigs.MachineConfig) (func() error, func() error, error) {
	bl := mc.LibKrunHypervisor.KRun.VirtualMachine.Bootloader
	if bl == nil {
		return nil, nil, fmt.Errorf("unable to determine boot loader for this machine")
	}
	return apple.StartGenericAppleVM(mc, krunkitBinary, bl, mc.LibKrunHypervisor.KRun.Endpoint)
}

func (l *LibKrunStubber) State(mc *vmconfigs.MachineConfig, bypass bool) (define.Status, error) {
	return mc.LibKrunHypervisor.KRun.State()
}

func (l *LibKrunStubber) StopVM(mc *vmconfigs.MachineConfig, hardStop bool) error {
	return mc.LibKrunHypervisor.KRun.Stop(hardStop, true)
}

func (l *LibKrunStubber) StopHostNetworking(mc *vmconfigs.MachineConfig, vmType define.VMType) error {
	return nil
}

func (l *LibKrunStubber) VMType() define.VMType {
	return define.LibKrun
}

func (l *LibKrunStubber) UserModeNetworkEnabled(mc *vmconfigs.MachineConfig) bool {
	return true
}

func (l *LibKrunStubber) UseProviderNetworkSetup() bool {
	return false
}

func (l *LibKrunStubber) RequireExclusiveActive() bool {
	return true
}

func (l *LibKrunStubber) UpdateSSHPort(mc *vmconfigs.MachineConfig, port int) error {
	return nil
}

func (l *LibKrunStubber) GetRosetta(mc *vmconfigs.MachineConfig) (bool, error) {
	return false, nil
}
