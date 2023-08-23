package types

import (
	"net"
	"regexp"
)

type Configuration struct {
	// Print packets on stderr
	Debug bool

	// Record all packets coming in and out in a file that can be read by Wireshark (pcap)
	CaptureFile string

	// Length of packet
	// Larger packets means less packets to exchange for the same amount of data (and less protocol overhead)
	MTU int

	// Network reserved for the virtual network
	Subnet string

	// IP address of the virtual gateway
	GatewayIP string

	// MAC address of the virtual gateway
	GatewayMacAddress string

	// Built-in DNS records that will be served by the DNS server embedded in the gateway
	DNS []Zone

	// List of search domains that will be added in all DHCP replies
	DNSSearchDomains []string

	// Port forwarding between the machine running the gateway and the virtual network.
	Forwards map[string]string

	// Address translation of incoming traffic.
	// Useful for reaching the host itself (localhost) from the virtual network.
	NAT map[string]string

	// IPs assigned to the gateway that can answer to ARP requests
	GatewayVirtualIPs []string

	// DHCP static leases. Allow to assign pre-defined IP to virtual machine based on the MAC address
	DHCPStaticLeases map[string]string

	// Only for Hyperkit
	// Allow to assign a pre-defined MAC address to an Hyperkit VM
	VpnKitUUIDMacAddresses map[string]string

	// Protocol to be used. Only for /connect mux
	Protocol Protocol
}

type Protocol string

const (
	// HyperKitProtocol is handshake, then 16bits little endian size of packet, then the packet.
	HyperKitProtocol Protocol = "hyperkit"
	// QemuProtocol is 32bits big endian size of the packet, then the packet.
	QemuProtocol Protocol = "qemu"
	// BessProtocol transfers bare L2 packets as SOCK_SEQPACKET.
	BessProtocol Protocol = "bess"
	// StdioProtocol is HyperKitProtocol without the handshake
	StdioProtocol Protocol = "stdio"
	// VfkitProtocol transfers bare L2 packets as SOCK_DGRAM.
	VfkitProtocol Protocol = "vfkit"
)

type Zone struct {
	Name      string
	Records   []Record
	DefaultIP net.IP
}

type Record struct {
	Name   string
	IP     net.IP
	Regexp *regexp.Regexp
}
