//go:build darwin
// +build darwin

package applehv

import (
	"github.com/containers/podman/v4/pkg/machine"
	vfConfig "github.com/crc-org/vfkit/pkg/config"
)

func getDefaultDevices(imagePath, logPath, readyPath string) ([]vfConfig.VirtioDevice, error) {
	var devices []vfConfig.VirtioDevice

	disk, err := vfConfig.VirtioBlkNew(imagePath)
	if err != nil {
		return nil, err
	}
	rng, err := vfConfig.VirtioRngNew()
	if err != nil {
		return nil, err
	}

	serial, err := vfConfig.VirtioSerialNew(logPath)
	if err != nil {
		return nil, err
	}

	readyDevice, err := vfConfig.VirtioVsockNew(1025, readyPath, true)
	if err != nil {
		return nil, err
	}
	devices = append(devices, disk, rng, serial, readyDevice)
	return devices, nil
}

func getDebugDevices() ([]vfConfig.VirtioDevice, error) {
	var devices []vfConfig.VirtioDevice
	gpu, err := vfConfig.VirtioGPUNew()
	if err != nil {
		return nil, err
	}
	mouse, err := vfConfig.VirtioInputNew(vfConfig.VirtioInputPointingDevice)
	if err != nil {
		return nil, err
	}
	kb, err := vfConfig.VirtioInputNew(vfConfig.VirtioInputKeyboardDevice)
	if err != nil {
		return nil, err
	}
	return append(devices, gpu, mouse, kb), nil
}

func getIgnitionVsockDevice(path string) (vfConfig.VirtioDevice, error) {
	return vfConfig.VirtioVsockNew(1024, path, true)
}

func VirtIOFsToVFKitVirtIODevice(fs machine.VirtIoFs) vfConfig.VirtioFs {
	return vfConfig.VirtioFs{
		DirectorySharingConfig: vfConfig.DirectorySharingConfig{
			MountTag: fs.Tag,
		},
		SharedDir: fs.Source,
	}
}
