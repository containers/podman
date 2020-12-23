package network

/*
	The code in this was kindly contributed by Dan Williams(dcbw@redhat.com).  Many thanks
	for his contributions.
*/

import (
	"fmt"
	"net"
)

func incByte(subnet *net.IPNet, idx int, shift uint) error {
	if idx < 0 {
		return fmt.Errorf("no more subnets left")
	}
	if subnet.IP[idx] == 255 {
		subnet.IP[idx] = 0
		return incByte(subnet, idx-1, 0)
	}
	subnet.IP[idx] += 1 << shift
	return nil
}

// NextSubnet returns subnet incremented by 1
func NextSubnet(subnet *net.IPNet) (*net.IPNet, error) {
	newSubnet := &net.IPNet{
		IP:   subnet.IP,
		Mask: subnet.Mask,
	}
	ones, bits := newSubnet.Mask.Size()
	if ones == 0 {
		return nil, fmt.Errorf("%s has only one subnet", subnet.String())
	}
	zeroes := uint(bits - ones)
	shift := zeroes % 8
	idx := ones/8 - 1
	if idx < 0 {
		idx = 0
	}
	if err := incByte(newSubnet, idx, shift); err != nil {
		return nil, err
	}
	return newSubnet, nil
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
