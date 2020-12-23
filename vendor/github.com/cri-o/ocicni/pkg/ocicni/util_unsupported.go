// +build !linux

package ocicni

import (
	"errors"
	"net"
)

type nsManager struct {
}

var errUnsupportedPlatform = errors.New("unsupported platform")

func (nsm *nsManager) init() error {
	return nil
}

func getContainerDetails(nsm *nsManager, netnsPath, interfaceName, addrType string) (*net.IPNet, *net.HardwareAddr, error) {
	return nil, nil, errUnsupportedPlatform
}

func tearDownLoopback(netns string) error {
	return errUnsupportedPlatform
}

func bringUpLoopback(netns string) error {
	return errUnsupportedPlatform
}

func checkLoopback(netns string) error {
	return errUnsupportedPlatform

}
