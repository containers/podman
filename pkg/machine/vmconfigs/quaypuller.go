package vmconfigs

import (
	"fmt"
	"runtime"

	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/env"
	"go.podman.io/podman/v6/pkg/machine/shim/diskpull"
	"go.podman.io/image/v5/types"
)

type QuayPuller struct {
	localPath     *define.VMFile
	sourceURI     string
	vmType        define.VMType
	machineConfig *MachineConfig
	machineDirs   *define.MachineDirs
	skipTlsVerify types.OptionalBool
}

func NewQuayPuller(vmType define.VMType, mc *MachineConfig, skipTlsVerify types.OptionalBool) (*QuayPuller, error) {
	puller := QuayPuller{
		vmType:        vmType,
		machineConfig: mc,
		skipTlsVerify: skipTlsVerify,
	}

	dirs, err := env.GetMachineDirs(vmType)
	if err != nil {
		return nil, err
	}
	puller.machineDirs = dirs

	return &puller, nil
}

func (puller QuayPuller) SetSourceURI(uri string) {
	puller.sourceURI = uri
}

func imageExtension(vmType define.VMType) string {
	switch vmType {
	case define.QemuVirt:
		return ".qcow2"
	case define.AppleHvVirt, define.LibKrun:
		return ".raw"
	case define.HyperVVirt:
		return ".vhdx"
	case define.WSLVirt:
		return ""
	default:
		return ""
	}
}

func localImagePath(machineDirs *define.MachineDirs, name string, imageExtension string) (*define.VMFile, error) {
	return machineDirs.DataDir.AppendToNewVMFile(fmt.Sprintf("%s-%s%s", name, runtime.GOARCH, imageExtension), nil)
}

func (puller QuayPuller) LocalPath() (*define.VMFile, error) {
	return localImagePath(puller.machineDirs, puller.machineConfig.Name, imageExtension(puller.vmType))
}

func (puller QuayPuller) Download() error {
	imagePath, err := puller.LocalPath()
	if err != nil {
		return err
	}
	return diskpull.GetDisk(puller.sourceURI, puller.machineDirs, imagePath, puller.vmType, puller.machineConfig.Name, puller.skipTlsVerify)
}
