package network

import (
	"fmt"
	"os/exec"

	"github.com/containers/libpod/pkg/util"
	"github.com/containers/libpod/utils"
	"github.com/sirupsen/logrus"
)

// GetFreeDeviceName returns a device name that is unused; used when no network
// name is provided by user
func GetFreeDeviceName() (string, error) {
	var (
		deviceNum  uint
		deviceName string
	)
	networkNames, err := GetNetworkNamesFromFileSystem()
	if err != nil {
		return "", err
	}
	liveNetworksNames, err := GetLiveNetworkNames()
	if err != nil {
		return "", err
	}
	for {
		deviceName = fmt.Sprintf("%s%d", CNIDeviceName, deviceNum)
		logrus.Debugf("checking if device name %s exists in other cni networks", deviceName)
		if util.StringInSlice(deviceName, networkNames) {
			deviceNum++
			continue
		}
		logrus.Debugf("checking if device name %s exists in live networks", deviceName)
		if !util.StringInSlice(deviceName, liveNetworksNames) {
			break
		}
		// TODO Still need to check the bridge names for a conflict but I dont know
		// how to get them yet!
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
