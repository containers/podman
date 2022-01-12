package util

import (
	"crypto/rand"
	"net"

	"github.com/pkg/errors"
)

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

// getRandomIPv6Subnet returns a random internal ipv6 subnet as described in RFC3879.
func getRandomIPv6Subnet() (net.IPNet, error) {
	ip := make(net.IP, 8, net.IPv6len)
	// read 8 random bytes
	_, err := rand.Read(ip)
	if err != nil {
		return net.IPNet{}, err
	}
	// first byte must be FD as per RFC3879
	ip[0] = 0xfd
	// add 8 zero bytes
	ip = append(ip, make([]byte, 8)...)
	return net.IPNet{IP: ip, Mask: net.CIDRMask(64, 128)}, nil
}
