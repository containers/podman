//go:build !windows

package cloudinit

import (
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
	"gopkg.in/yaml.v3"
)

func GenerateUserData(mc *vmconfigs.MachineConfig) ([]byte, error) {
	sshKey, err := machine.GetSSHKeys(mc.SSH.IdentityPath)
	if err != nil {
		return nil, err
	}

	userData := UserData{
		Users: []User{
			User{
				Name:    mc.SSH.RemoteUsername,
				Sudo:    "ALL=(ALL) NOPASSWD:ALL",
				Shell:   "/bin/bash",
				Groups:  []string{"users"},
				SSHKeys: []string{sshKey},
			},
		},
	}

	yamlBytes, err := yaml.Marshal(&userData)
	if err != nil {
		logrus.Errorf("Error marshaling to YAML: %v", err)
		return nil, err
	}

	headerLine := "#cloud-config\n"
	yamlBytes = append([]byte(headerLine), yamlBytes...)

	return yamlBytes, nil
}

func GetEmbeddedResources(_ *vmconfigs.MachineConfig) []EmbeddedResource {
	return nil
}
