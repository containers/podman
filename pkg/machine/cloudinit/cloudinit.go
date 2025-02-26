package cloudinit

import (
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type User struct {
	Name    string   `yaml:"name"`
	Sudo    string   `yaml:"sudo"`
	Shell   string   `yaml:"shell"`
	Groups  string   `yaml:"groups"`
	SSHKeys []string `yaml:"ssh_authorized_keys"`
}

type UserData struct {
	Users []User `yaml:"users"`
}

func GenerateUserData(dir string, data UserData) (string, error) {
	yamlBytes, err := yaml.Marshal(&data)
	if err != nil {
		logrus.Errorf("Error marshaling to YAML: %v", err)
		return "", err
	}

	headerLine := "#cloud-config\n"
	yamlBytes = append([]byte(headerLine), yamlBytes...)

	path := filepath.Join(dir, "user-data")
	err = os.WriteFile(path, yamlBytes, 0644)
	if err != nil {
		logrus.Errorf("Error creating temp file: %v", err)
		return "", err
	}
	return path, nil
}
