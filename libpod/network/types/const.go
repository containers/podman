package types

const (
	// BridgeNetworkDriver defines the bridge driver
	BridgeNetworkDriver = "bridge"
	// DefaultNetworkDriver is the default network type used
	DefaultNetworkDriver = BridgeNetworkDriver
	// MacVLANNetworkDriver defines the macvlan driver
	MacVLANNetworkDriver = "macvlan"

	// IPAM drivers
	// HostLocalIPAMDriver store the ip
	HostLocalIPAMDriver = "host-local"
	// DHCPIPAMDriver get subnet and ip from dhcp server
	DHCPIPAMDriver = "dhcp"

	// DefaultSubnet is the name that will be used for the default CNI network.
	DefaultNetworkName = "podman"
	// DefaultSubnet is the subnet that will be used for the default CNI network.
	DefaultSubnet = "10.88.0.0/16"
)
