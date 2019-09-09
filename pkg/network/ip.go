package network

import (
	"net"

	"github.com/containernetworking/plugins/pkg/ip"
)

// CalcGatewayIP takes a network and returns the first IP in it.
func CalcGatewayIP(ipn *net.IPNet) net.IP {
	// taken from cni bridge plugin as it is not exported
	nid := ipn.IP.Mask(ipn.Mask)
	return ip.NextIP(nid)
}
