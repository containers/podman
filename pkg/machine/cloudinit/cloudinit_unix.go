//go:build !windows

package cloudinit

import (
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

func generateDefaultUserData(mc *vmconfigs.MachineConfig) ([]byte, error) {
	userData, err := defaultUserData(mc)
	if err != nil {
		return nil, err
	}

	return userData.Marshal()
}

func generateUserData(mc *vmconfigs.MachineConfig) ([]byte, error) {
	// If user has not provided any custom user-data, generate default
	// otherwise use the provided one
	if mc.CloudInitConfig.UserData == nil {
		return generateDefaultUserData(mc)
	}

	return mc.CloudInitConfig.UserData.Read()
}

func GetEmbeddedResources(_ *vmconfigs.MachineConfig) []EmbeddedResource {
	return nil
}
