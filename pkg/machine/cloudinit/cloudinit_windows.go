//go:build windows

package cloudinit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/hyperv/hutil"
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

	if mc.HyperVHypervisor != nil && mc.HyperVHypervisor.UserModeNetworking {
		netUnitFile, err := hutil.CreateNetworkUnit(mc.HyperVHypervisor.NetworkVSock.Port)
		if err != nil {
			return nil, err
		}

		userData.WriteFiles = []WriteFile{
			{
				Path:        "/etc/NetworkManager/system-connections/vsock0.nmconnection",
				Content:     hutil.HyperVVsockNMConnection,
				Permissions: "0600",
				Owner:       "root",
			},
			{
				Path:        "/etc/systemd/system/vsock-network.service",
				Content:     netUnitFile,
				Permissions: "0644",
				Owner:       "root",
			},
		}

		userData.RunCmd = []string{
			"install -o root -g root -m 0755 /mnt/gvforwarder /usr/local/bin/gvforwarder",
			"nmcli connection reload",
			"systemctl daemon-reload",
			"systemctl enable --now vsock-network.service",
		}

		userData.Mounts = [][]string{
			{"/dev/sr0", "/mnt", "iso9660", "defaults,ro", "0", "0"},
		}
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

func getGvForwarderBytes() ([]byte, error) {
	url, err := getGvForwarderDownloadUrl()
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download gvforwarder: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			logrus.Error(err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download gvforwarder: %s", resp.Status)
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read gvforwarder response body: %w", err)
	}

	return bytes, nil
}

func getGvForwarderDownloadUrl() (string, error) {
	apiURL := "https://api.github.com/repos/containers/gvisor-tap-vsock/releases/latest"
	resp, err := http.Get(apiURL)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logrus.Error(err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get latest gvforwarder release information: %s", resp.Status)
	}

	var releaseInfo struct {
		Assets []struct {
			Name        string `json:"name"`
			DownloadUrl string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releaseInfo); err != nil {
		return "", err
	}

	for _, asset := range releaseInfo.Assets {
		if asset.Name == "gvforwarder" {
			return asset.DownloadUrl, nil
		}
	}

	return "", fmt.Errorf("gvforwarder latest release not found")
}

func GetEmbeddedResources(mc *vmconfigs.MachineConfig) []EmbeddedResource {
	// Only add gvforwarder if using Hyper-V with user-mode networking
	if mc.HyperVHypervisor == nil || !mc.HyperVHypervisor.UserModeNetworking {
		return nil
	}

	extraFiles := []EmbeddedResource{}
	gvforwarderBytes, err := getGvForwarderBytes()
	if err != nil {
		logrus.Errorf("Failed to get gvforwarder: %v", err)
		return extraFiles
	}
	extraFiles = append(extraFiles, EmbeddedResource{
		Name:    "gvforwarder",
		Content: gvforwarderBytes,
	})
	return extraFiles
}
