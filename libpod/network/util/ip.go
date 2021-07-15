package util

import (
	"crypto/rand"
	"net"

	"github.com/pkg/errors"
)

// IsIPv6 returns true if netIP is IPv6.
func IsIPv6(netIP net.IP) bool {
	return netIP != nil && netIP.To4() == nil
}

// IsIPv4 returns true if netIP is IPv4.
func IsIPv4(netIP net.IP) bool {
	return netIP != nil && netIP.To4() != nil
}

func incByte(subnet *net.IPNet, idx int, shift uint) error {
	if idx < 0 {
		return errors.New("no more subnets left")
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
		return nil, errors.Errorf("%s has only one subnet", subnet.String())
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

func NetworkIntersectsWithNetworks(n *net.IPNet, networklist []*net.IPNet) bool {
	for _, nw := range networklist {
		if networkIntersect(n, nw) {
			return true
		}
	}
	return false
}

func networkIntersect(n1, n2 *net.IPNet) bool {
	return n2.Contains(n1.IP) || n1.Contains(n2.IP)
}

// GetRandomIPv6Subnet returns a random internal ipv6 subnet as described in RFC3879.
func GetRandomIPv6Subnet() (net.IPNet, error) {
	ip := make(net.IP, 8, net.IPv6len)
	// read 8 random bytes
	_, err := rand.Read(ip)
	if err != nil {
		return net.IPNet{}, nil
	}
	// first byte must be FD as per RFC3879
	ip[0] = 0xfd
	// add 8 zero bytes
	ip = append(ip, make([]byte, 8)...)
	return net.IPNet{IP: ip, Mask: net.CIDRMask(64, 128)}, nil
}
