// +build !linux

package ocicni

import (
	"fmt"
	"net"
)

type nsManager struct {
}

func (nsm *nsManager) init() error {
	return nil
}

func getContainerDetails(nsm *nsManager, netnsPath, interfaceName, addrType string) (*net.IPNet, *net.HardwareAddr, error) {
	return nil, nil, fmt.Errorf("not supported yet")
}
