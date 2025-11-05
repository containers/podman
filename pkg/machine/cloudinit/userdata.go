package cloudinit

import (
	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
	"gopkg.in/yaml.v3"
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

func defaultUserData(mc *vmconfigs.MachineConfig) (*UserData, error) {
	sshKey, err := machine.GetSSHKeys(mc.SSH.IdentityPath)
	if err != nil {
		return nil, err
	}

	return &UserData{
		Users: []User{
			{
				Name:    mc.SSH.RemoteUsername,
				Sudo:    "ALL=(ALL) NOPASSWD:ALL",
				Shell:   "/bin/bash",
				Groups:  []string{"users"},
				SSHKeys: []string{sshKey},
			},
		},
	}, nil
}

func (userData *UserData) Marshal() ([]byte, error) {
	data, err := yaml.Marshal(userData)
	if err != nil {
		return nil, err
	}

	headerLine := "#cloud-config\n"
	data = append([]byte(headerLine), data...)

	return data, nil
}
