//go:build !windows

package cloudinit

import (
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
	"gopkg.in/yaml.v3"
)

func generateDefaultUserData(mc *vmconfigs.MachineConfig) ([]byte, error) {
	userData, err := getDefaultUserData(mc)
	if err != nil {
		return nil, err
	}

	userDataBytes, err := yaml.Marshal(&userData)
	if err != nil {
		logrus.Errorf("Error marshaling to YAML: %v", err)
		return nil, err
	}

	headerLine := "#cloud-config\n"
	userDataBytes = append([]byte(headerLine), userDataBytes...)

	return userDataBytes, nil
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
