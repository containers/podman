package slirp4netns

import "net"

const (
	ipv6ConfDefaultAcceptDadSysctl = "/proc/sys/net/ipv6/conf/default/accept_dad"
	BinaryName                     = "slirp4netns"

	// defaultMTU the default MTU override
	defaultMTU = 65520

	// default slirp4ns subnet
	defaultSubnet = "10.0.2.0/24"
)

// SetupResult return type from Setup()
type SetupResult struct {
	// Pid of the created slirp4netns process
	Pid int
	// Subnet which is used by slirp4netns
	Subnet *net.IPNet
	// IPv6 whenever Ipv6 is enabled in slirp4netns
	IPv6 bool
}
