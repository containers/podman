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

func addMountsSupport(userData *UserData, mc *vmconfigs.MachineConfig) error {
	// Create systemd template service for 9p vsock proxying
	// Template units use %i as the instance identifier (port number)
	// This allows us to instantiate one unit per mount without duplication
	//
	// We use socat to actively connect to the host's vsock and proxy to a Unix socket.
	serviceTemplate := `[Unit]
Description=9p VSOCK to Unix Socket Proxy for port %i
After=network.target

[Service]
Type=simple
# Use socat to actively CONNECT to host's vsock port and create Unix socket
# VSOCK-CONNECT:2:%i connects to host CID 2 on port %i
# UNIX-LISTEN:/run/9p-%i.sock creates Unix socket for mount operations
# Options: -ly enables logging, umask=000 makes socket world-accessible,
# reuseaddr allows socket reuse, keepalive maintains connection health
ExecStop=/bin/rm -f /run/9p-%i.sock
ExecStart=/usr/bin/socat -ly UNIX-LISTEN:/run/9p-%i.sock,umask=000,reuseaddr VSOCK-CONNECT:2:%i,keepalive
Restart=on-failure
RestartSec=5
# Ensure service runs as root (default, but explicit for clarity)
User=root

[Install]
WantedBy=multi-user.target
`

	// Add the template service unit file
	userData.AddWriteFiles([]WriteFile{
		WriteFile{
			Path:        "/etc/systemd/system/9p-vsock@.service",
			Content:     serviceTemplate,
			Permissions: "0644",
			Owner:       "root",
		},
	})

	// Add commands to reload systemd and enable the service instances
	// Ensure /run exists for Unix sockets
	enableCmds := []string{
		"mkdir -p /run",
		"systemctl daemon-reload",
		"dnf install -y socat",
	}
	for _, mount := range mc.Mounts {
		if mount.VSockNumber != nil {
			instanceName := fmt.Sprintf("9p-vsock@%d.service", *mount.VSockNumber)
			enableCmds = append(enableCmds, fmt.Sprintf("systemctl enable --now %s", instanceName))
		}
	}

	userData.AddRunCmds(enableCmds)

	return nil
}

func addAdditionalUserData(userData *UserData, mc *vmconfigs.MachineConfig) error {
	if mc.HyperVHypervisor != nil && mc.HyperVHypervisor.UserModeNetworking {
		err := addUserModeNetworking(userData, mc)
		if err != nil {
			return err
		}
	}

	if len(mc.Mounts) > 0 {
		err := addMountsSupport(userData, mc)
		if err != nil {
			return err
		}
	}
	return nil
}

func defaultHypervUserData(mc *vmconfigs.MachineConfig) ([]byte, error) {
	userData, err := defaultUserData(mc)
	if err != nil {
		return nil, err
	}

	err = addAdditionalUserData(userData, mc)
	if err != nil {
		return nil, err
	}

	return userData.Marshal()
}

func generateUserData(mc *vmconfigs.MachineConfig) ([]byte, error) {
	var err error
	userModeNetworking := mc.HyperVHypervisor != nil && mc.HyperVHypervisor.UserModeNetworking

	// If user has not provided any custom user-data, generate default
	if mc.CloudInitConfig.UserData == nil {
		return defaultHypervUserData(mc)
	}

	customUserData, err := mc.CloudInitConfig.UserData.Read()
	if err != nil {
		return nil, err
	}

	// if user has provided a custom user-data but we're not on Hyper-V/user-mode networking
	// and there are no mounts, return it as-is
	if !userModeNetworking && len(mc.Mounts) == 0 {
		return customUserData, nil
	}

	// otherwise use the custom user-data and add the user-mode networking configuration or 9p mounts support
	generatedUserData := &UserData{}

	err = addAdditionalUserData(generatedUserData, mc)
	if err != nil {
		return nil, err
	}

	// if the user has provided a custom user-data and we are on Hyper-V/user-mode networking,
	// we need to merge our generated user data with user's one
	// To do it we create a MIME multi-part archive
	// with both files
	return generatedUserData.MarshalMultiPart(customUserData)
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
