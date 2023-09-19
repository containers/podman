package types

type TransportProtocol string

const (
	UDP   TransportProtocol = "udp"
	TCP   TransportProtocol = "tcp"
	UNIX  TransportProtocol = "unix"
	NPIPE TransportProtocol = "npipe"
)

type ExposeRequest struct {
	Local    string            `json:"local"`
	Remote   string            `json:"remote"`
	Protocol TransportProtocol `json:"protocol"`
}

type UnexposeRequest struct {
	Local    string            `json:"local"`
	Protocol TransportProtocol `json:"protocol"`
}
