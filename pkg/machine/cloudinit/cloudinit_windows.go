//go:build windows

package cloudinit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/machine/hyperv/hutil"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

func addUserModeNetworking(userData *UserData, mc *vmconfigs.MachineConfig) error {
	netUnitFile, err := hutil.CreateNetworkUnitWithBinary("/usr/local/bin/gvforwarder", mc.HyperVHypervisor.NetworkVSock.Port)
	if err != nil {
		return err
	}

	userData.AddWriteFiles([]WriteFile{
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
	})

	userData.AddRunCmds([]string{
		"install -o root -g root -m 0755 /mnt/gvforwarder /usr/local/bin/gvforwarder",
		"nmcli connection reload",
		"systemctl daemon-reload",
		"systemctl enable --now vsock-network.service",
	})

	userData.Mounts = [][]string{
		{"/dev/sr0", "/mnt", "iso9660", "defaults,ro", "0", "0"},
	}

	return nil
}

func defaultHypervUserData(mc *vmconfigs.MachineConfig, userModeNetworking bool) ([]byte, error) {
	userData, err := defaultUserData(mc)
	if err != nil {
		return nil, err
	}

	if userModeNetworking {
		err = addUserModeNetworking(userData, mc)
		if err != nil {
			return nil, err
		}
	}

	return userData.Marshal()
}

func generateUserData(mc *vmconfigs.MachineConfig) ([]byte, error) {
	var err error
	userModeNetworking := mc.HyperVHypervisor != nil && mc.HyperVHypervisor.UserModeNetworking

	// If user has not provided any custom user-data, generate default
	if mc.CloudInitConfig.UserData == nil {
		return defaultHypervUserData(mc, userModeNetworking)
	}

	customUserData, err := mc.CloudInitConfig.UserData.Read()
	if err != nil {
		return nil, err
	}

	// if user has provided a custom user-data but we're not on Hyper-V/user-mode networking, return it as-is
	if !userModeNetworking {
		return customUserData, nil
	}

	// otherwise use the custom user-data and add the user-mode networking configuration
	userModeNetworkingUserData := &UserData{}

	if err := addUserModeNetworking(userModeNetworkingUserData, mc); err != nil {
		return nil, err
	}

	// if the user has provided a custom user-data and we are on Hyper-V/user-mode networking,
	// we need to merge our generated user data with user's one
	// To do it we create a MIME multi-part archive
	// with both files
	return userModeNetworkingUserData.MarshalMultiPart(customUserData)
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
