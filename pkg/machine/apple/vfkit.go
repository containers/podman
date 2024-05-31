//go:build darwin

package apple

import (
	"errors"

	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	vfConfig "github.com/crc-org/vfkit/pkg/config"
	"github.com/crc-org/vfkit/pkg/rest"
)

func GetDefaultDevices(mc *vmconfigs.MachineConfig) ([]vfConfig.VirtioDevice, *define.VMFile, error) {
	var devices []vfConfig.VirtioDevice

	disk, err := vfConfig.VirtioBlkNew(mc.ImagePath.GetPath())
	if err != nil {
		return nil, nil, err
	}
	rng, err := vfConfig.VirtioRngNew()
	if err != nil {
		return nil, nil, err
	}

	logfile, err := mc.LogFile()
	if err != nil {
		return nil, nil, err
	}
	serial, err := vfConfig.VirtioSerialNew(logfile.GetPath())
	if err != nil {
		return nil, nil, err
	}

	readySocket, err := mc.ReadySocket()
	if err != nil {
		return nil, nil, err
	}

	readyDevice, err := vfConfig.VirtioVsockNew(1025, readySocket.GetPath(), true)
	if err != nil {
		return nil, nil, err
	}
	devices = append(devices, disk, rng, serial, readyDevice)

	if mc.AppleHypervisor != nil && mc.AppleHypervisor.Vfkit.Rosetta {
		rosetta := &vfConfig.RosettaShare{
			DirectorySharingConfig: vfConfig.DirectorySharingConfig{
				MountTag: define.MountTag,
			},
			InstallRosetta: true,
		}
		devices = append(devices, rosetta)
	}

	return devices, readySocket, nil
}

func GetDebugDevices() ([]vfConfig.VirtioDevice, error) {
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

func GetIgnitionVsockDevice(path string) (vfConfig.VirtioDevice, error) {
	return vfConfig.VirtioVsockNew(1024, path, true)
}

func VirtIOFsToVFKitVirtIODevice(mounts []*vmconfigs.Mount) ([]vfConfig.VirtioDevice, error) {
	virtioDevices := make([]vfConfig.VirtioDevice, 0, len(mounts))
	for _, vol := range mounts {
		virtfsDevice, err := vfConfig.VirtioFsNew(vol.Source, vol.Tag)
		if err != nil {
			return nil, err
		}
		virtioDevices = append(virtioDevices, virtfsDevice)
	}
	return virtioDevices, nil
}

// GetVfKitEndpointCMDArgs converts the vfkit endpoint to a cmdline format
func GetVfKitEndpointCMDArgs(endpoint string) ([]string, error) {
	if len(endpoint) == 0 {
		return nil, errors.New("endpoint cannot be empty")
	}
	restEndpoint, err := rest.NewEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	return restEndpoint.ToCmdLine()
}

// GetIgnitionVsockDeviceAsCLI retrieves the ignition vsock device and converts
// it to a cmdline format
func GetIgnitionVsockDeviceAsCLI(ignitionSocketPath string) ([]string, error) {
	ignitionVsockDevice, err := GetIgnitionVsockDevice(ignitionSocketPath)
	if err != nil {
		return nil, err
	}
	// Convert the device into cli args
	ignitionVsockDeviceCLI, err := ignitionVsockDevice.ToCmdLine()
	if err != nil {
		return nil, err
	}
	return ignitionVsockDeviceCLI, nil
}

// GetDebugDevicesCMDArgs retrieves the debug devices and converts them to a
// cmdline format
func GetDebugDevicesCMDArgs() ([]string, error) {
	args := []string{}
	debugDevices, err := GetDebugDevices()
	if err != nil {
		return nil, err
	}
	for _, debugDevice := range debugDevices {
		debugCli, err := debugDevice.ToCmdLine()
		if err != nil {
			return nil, err
		}
		args = append(args, debugCli...)
	}
	return args, nil
}
