// +build linux

package ocicni

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

var defaultNamespaceEnterCommandName = "nsenter"

type nsManager struct {
	nsenterPath string
}

func (nsm *nsManager) init() error {
	var err error
	nsm.nsenterPath, err = exec.LookPath(defaultNamespaceEnterCommandName)
	return err
}

func getContainerDetails(nsm *nsManager, netnsPath, interfaceName, addrType string) (*net.IPNet, *net.HardwareAddr, error) {
	// Try to retrieve ip inside container network namespace
	output, err := exec.Command(nsm.nsenterPath, fmt.Sprintf("--net=%s", netnsPath), "-F", "--",
		"ip", "-o", addrType, "addr", "show", "dev", interfaceName, "scope", "global").CombinedOutput()
	if err != nil {
		return nil, nil, fmt.Errorf("Unexpected command output %s with error: %v", output, err)
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 1 {
		return nil, nil, fmt.Errorf("Unexpected command output %s", output)
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 4 {
		return nil, nil, fmt.Errorf("Unexpected address output %s ", lines[0])
	}
	ip, ipNet, err := net.ParseCIDR(fields[3])
	if err != nil {
		return nil, nil, fmt.Errorf("CNI failed to parse ip from output %s due to %v", output, err)
	}
	if ip.To4() == nil {
		ipNet.IP = ip
	} else {
		ipNet.IP = ip.To4()
	}

	// Try to retrieve MAC inside container network namespace
	output, err = exec.Command(nsm.nsenterPath, fmt.Sprintf("--net=%s", netnsPath), "-F", "--",
		"ip", "link", "show", "dev", interfaceName).CombinedOutput()
	if err != nil {
		return nil, nil, fmt.Errorf("unexpected 'ip link' command output %s with error: %v", output, err)
	}

	lines = strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return nil, nil, fmt.Errorf("unexpected 'ip link' command output %s", output)
	}
	fields = strings.Fields(lines[1])
	if len(fields) < 4 {
		return nil, nil, fmt.Errorf("unexpected link output %s ", lines[0])
	}
	mac, err := net.ParseMAC(fields[1])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse MAC from output %s due to %v", output, err)
	}

	return ipNet, &mac, nil
}
