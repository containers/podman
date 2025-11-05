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
)

type User struct {
	Name    string   `yaml:"name"`
	Sudo    string   `yaml:"sudo"`
	Shell   string   `yaml:"shell"`
	Groups  []string `yaml:"groups"`
	SSHKeys []string `yaml:"ssh_authorized_keys"`
}

type WriteFile struct {
	Path        string `yaml:"path,omitempty"`
	Content     string `yaml:"content,omitempty"`
	Encoding    string `yaml:"encoding,omitempty"`
	Owner       string `yaml:"owner,omitempty"`
	Permissions string `yaml:"permissions,omitempty"`
}

type UserData struct {
	Users      []User      `yaml:"users"`
	WriteFiles []WriteFile `yaml:"write_files,omitempty"`
	RunCmd     []string    `yaml:"runcmd,omitempty"`
	Mounts     [][]string  `yaml:"mounts,omitempty"`
}

type EmbeddedResource struct {
	Name    string `yaml:"name"`
	Content []byte `yaml:"content"`
}

func GenerateUserDataFile(mc *vmconfigs.MachineConfig) (string, error) {
	yamlBytes, err := generateUserData(mc)
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

	metadata, networkConfig := []byte{}, []byte{}

	userdata, err := generateUserData(mc)
	if err != nil {
		return nil, fmt.Errorf("failed to generate user-data file: %w", err)
	}

	if mc.CloudInitConfig.MetaData != nil {
		metadata, err = mc.CloudInitConfig.MetaData.Read()
		if err != nil {
			return nil, fmt.Errorf("failed to read meta-data file: %w", err)
		}
	}

	if mc.CloudInitConfig.NetworkConfig != nil {
		networkConfig, err = mc.CloudInitConfig.NetworkConfig.Read()
		if err != nil {
			return nil, fmt.Errorf("failed to read network-config file: %w", err)
		}
	}

	if err := writer.AddFile(bytes.NewReader(userdata), "user-data"); err != nil {
		return nil, err
	}
	if err := writer.AddFile(bytes.NewReader(metadata), "meta-data"); err != nil {
		return nil, err
	}
	if err := writer.AddFile(bytes.NewReader(networkConfig), "network-config"); err != nil {
		return nil, err
	}

	resources := GetEmbeddedResources(mc)
	if resources != nil {
		for _, res := range resources {
			if err := writer.AddFile(bytes.NewReader(res.Content), res.Name); err != nil {
				return nil, err
			}
		}
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

func getDefaultUserData(mc *vmconfigs.MachineConfig) (*UserData, error) {
	sshKey, err := machine.GetSSHKeys(mc.SSH.IdentityPath)
	if err != nil {
		return nil, err
	}

	return &UserData{
		Users: []User{
			User{
				Name:    mc.SSH.RemoteUsername,
				Sudo:    "ALL=(ALL) NOPASSWD:ALL",
				Shell:   "/bin/bash",
				Groups:  []string{"users"},
				SSHKeys: []string{sshKey},
			},
		},
	}, nil
}
