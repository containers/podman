package port

// PortMapping contains a set of ports that will be mapped into the container.
// Nolint applied for naming warning
// nolint
type PortMapping struct {
	// ContainerPort is the port in the container.
	// Must be a positive integer between 0 and 65535.
	ContainerPort int32 `json:"ctr"`
	// HostPort is the port on the host that will be forwarded to the container.
	// Must be a positive integer between 0 and 65535.
	HostPort int32 `json:"host"`
	// HostIP is the IP on the host that will have the port forwarded to
	// the container.
	HostIP string `json:"hostIP"`
	// Length is the number of ports that will be mapped.
	// Must be nonzero.
	// HostPort + Length and ContainerPort + Length must both be less than
	// 65536 to ensure that we do not try to map more ports than are
	// available.
	Length uint16 `json:"length"`
	// Protocol is the protocol that will be forwarded.
	// Valid protocols are "tcp" and "udp"
	Protocol string `json:"proto"`
}
