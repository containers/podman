package adapter

// DefaultAddress is the default address of the varlink socket
const DefaultAddress = "unix:/run/podman/io.podman"

// EndpointType declares the type of server connection
type EndpointType int

// Enum of connection types
const (
	Unknown          = iota - 1 // Unknown connection type
	BridgeConnection            // BridgeConnection proxy connection via ssh
	DirectConnection            // DirectConnection socket connection to server
)

// String prints ASCII string for EndpointType
func (e EndpointType) String() string {
	// declare an array of strings
	// ... operator counts how many
	// items in the array (7)
	names := [...]string{
		"BridgeConnection",
		"DirectConnection",
	}

	if e < BridgeConnection || e > DirectConnection {
		return "Unknown"
	}
	return names[e]
}

// Endpoint type and connection string to use
type Endpoint struct {
	Type       EndpointType
	Connection string
}
