package network

import (
	"fmt"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v3/pkg/util"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
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
	link, err := netlink.LinkByName(interfaceName)
	if err != nil {
		return err
	}
	return netlink.LinkDel(link)
}
