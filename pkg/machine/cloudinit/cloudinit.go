package cloudinit

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
	"github.com/kdomanski/iso9660"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type User struct {
	Name    string   `yaml:"name"`
	Sudo    string   `yaml:"sudo"`
	Shell   string   `yaml:"shell"`
	Groups  []string `yaml:"groups"`
	SSHKeys []string `yaml:"ssh_authorized_keys"`
}

type UserData struct {
	Users []User `yaml:"users"`
}

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

func GenerateUserDataFile(mc *vmconfigs.MachineConfig) (string, error) {
	yamlBytes, err := GenerateUserData(mc)
	if err != nil {
		return "", err
	}

	machineDataDir, err := mc.DataDir()
	if err != nil {
		return "", err
	}

	path := filepath.Join(machineDataDir.GetPath(), "user-data")
	// delete previous user-data, if any
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err := os.WriteFile(path, yamlBytes, 0644); err != nil {
		logrus.Errorf("Error writing cloudinit user-data file: %v", err)
		return "", err
	}
	return path, nil
}

func GetCloudInitISOVMFile(mc *vmconfigs.MachineConfig) (*define.VMFile, error) {
	machineDataDir, err := mc.DataDir()
	if err != nil {
		return nil, err
	}
	return machineDataDir.AppendToNewVMFile(fmt.Sprintf("%s-cloudinit.iso", mc.Name), nil)
}

func GenerateISO(mc *vmconfigs.MachineConfig) (*define.VMFile, error) {
	writer, err := iso9660.NewWriter()
	if err != nil {
		return nil, fmt.Errorf("failed to create writer: %w", err)
	}

	defer func() {
		if err := writer.Cleanup(); err != nil {
			logrus.Error(err)
		}
	}()

	userdata, err := GenerateUserData(mc)
	if err != nil {
		return nil, err
	}
	if err := writer.AddFile(bytes.NewReader(userdata), "user-data"); err != nil {
		return nil, err
	}
	if err := writer.AddFile(bytes.NewReader([]byte{}), "meta-data"); err != nil {
		return nil, err
	}

	vmFile, err := GetCloudInitISOVMFile(mc)
	if err != nil {
		return nil, err
	}

	isoFile, err := os.Create(vmFile.GetPath())
	if err != nil {
		return nil, fmt.Errorf("unable to create cloud-init ISO file: %w", err)
	}

	defer func() {
		if err := isoFile.Close(); err != nil {
			logrus.Error(fmt.Errorf("failed to close cloud-init ISO file: %w", err))
		}
	}()

	err = writer.WriteTo(isoFile, "cidata")
	if err != nil {
		os.Remove(isoFile.Name())
		return nil, fmt.Errorf("failed to write cloud-init ISO image: %w", err)
	}

	return vmFile, nil
}
