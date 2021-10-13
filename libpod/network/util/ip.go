package util

import "net"

// IsIPv6 returns true if netIP is IPv6.
func IsIPv6(netIP net.IP) bool {
	return netIP != nil && netIP.To4() == nil
}

// IsIPv4 returns true if netIP is IPv4.
func IsIPv4(netIP net.IP) bool {
	return netIP != nil && netIP.To4() != nil
}

// LastIPInSubnet gets the last IP in a subnet
func LastIPInSubnet(addr *net.IPNet) (net.IP, error) { //nolint:interfacer
	// re-parse to ensure clean network address
	_, cidr, err := net.ParseCIDR(addr.String())
	if err != nil {
		return nil, err
	}

	ones, bits := cidr.Mask.Size()
	if ones == bits {
		return cidr.IP, nil
	}
	for i := range cidr.IP {
		cidr.IP[i] = cidr.IP[i] | ^cidr.Mask[i]
	}
	return cidr.IP, nil
}

// FirstIPInSubnet gets the first IP in a subnet
func FirstIPInSubnet(addr *net.IPNet) (net.IP, error) { //nolint:interfacer
	// re-parse to ensure clean network address
	_, cidr, err := net.ParseCIDR(addr.String())
	if err != nil {
		return nil, err
	}
	ones, bits := cidr.Mask.Size()
	if ones == bits {
		return cidr.IP, nil
	}
	cidr.IP[len(cidr.IP)-1]++
	return cidr.IP, nil
}
