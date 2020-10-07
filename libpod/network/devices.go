package network

import (
	"fmt"
	"os/exec"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/containers/podman/v2/utils"
	"github.com/sirupsen/logrus"
)

// GetFreeDeviceName returns a device name that is unused; used when no network
// name is provided by user
func GetFreeDeviceName(config *config.Config) (string, error) {
	var (
		deviceNum  uint
		deviceName string
	)
	networkNames, err := GetNetworkNamesFromFileSystem(config)
	if err != nil {
		return "", err
	}
	liveNetworksNames, err := GetLiveNetworkNames()
	if err != nil {
		return "", err
	}
	bridgeNames, err := GetBridgeNamesFromFileSystem(config)
	if err != nil {
		return "", err
	}
	for {
		deviceName = fmt.Sprintf("%s%d", CNIDeviceName, deviceNum)
		logrus.Debugf("checking if device name %q exists in other cni networks", deviceName)
		if util.StringInSlice(deviceName, networkNames) {
			deviceNum++
			continue
		}
		logrus.Debugf("checking if device name %q exists in live networks", deviceName)
		if util.StringInSlice(deviceName, liveNetworksNames) {
			deviceNum++
			continue
		}
		logrus.Debugf("checking if device name %q already exists as a bridge name ", deviceName)
		if !util.StringInSlice(deviceName, bridgeNames) {
			break
		}
		deviceNum++
	}
	return deviceName, nil
}

// RemoveInterface removes an interface by the given name
func RemoveInterface(interfaceName string) error {
	// Make sure we have the ip command on the system
	ipPath, err := exec.LookPath("ip")
	if err != nil {
		return err
	}
	// Delete the network interface
	_, err = utils.ExecCmd(ipPath, []string{"link", "del", interfaceName}...)
	return err
}
