package ocicni

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

func getContainerDetails(nsenterPath, netnsPath, interfaceName, addrType string) (*net.IPNet, *net.HardwareAddr, error) {
	// Try to retrieve ip inside container network namespace
	output, err := exec.Command(nsenterPath, fmt.Sprintf("--net=%s", netnsPath), "-F", "--",
		"ip", "-o", addrType, "addr", "show", "dev", interfaceName, "scope", "global").CombinedOutput()
	if err != nil {
		return nil, nil, fmt.Errorf("unexpected 'ip addr' command output %s with error: %v", output, err)
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 1 {
		return nil, nil, fmt.Errorf("unexpected 'ip addr' command output %s", output)
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 4 {
		return nil, nil, fmt.Errorf("unexpected address output %s ", lines[0])
	}
	ip, ipNet, err := net.ParseCIDR(fields[3])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse IP from output %s due to %v", output, err)
	}
	if ip.To4() == nil {
		ipNet.IP = ip
	} else {
		ipNet.IP = ip.To4()
	}

	// Try to retrieve MAC inside container network namespace
	output, err = exec.Command(nsenterPath, fmt.Sprintf("--net=%s", netnsPath), "-F", "--",
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
