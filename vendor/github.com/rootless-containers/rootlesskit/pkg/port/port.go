package port

import (
	"context"
	"net"
)

type Spec struct {
	Proto      string `json:"proto,omitempty"`    // either "tcp" or "udp". in future "sctp" will be supported as well.
	ParentIP   string `json:"parentIP,omitempty"` // IPv4 address. can be empty (0.0.0.0).
	ParentPort int    `json:"parentPort,omitempty"`
	ChildPort  int    `json:"childPort,omitempty"`
	// ChildIP is an IPv4 address.
	// Default values:
	// - builtin     driver: 127.0.0.1
	// - socat       driver: 127.0.0.1
	// - slirp4netns driver: slirp4netns's child IP, e.g., 10.0.2.100
	ChildIP string `json:"childIP,omitempty"`
}

type Status struct {
	ID   int  `json:"id"`
	Spec Spec `json:"spec"`
}

// Manager MUST be thread-safe.
type Manager interface {
	AddPort(ctx context.Context, spec Spec) (*Status, error)
	ListPorts(ctx context.Context) ([]Status, error)
	RemovePort(ctx context.Context, id int) error
}

// ChildContext is used for RunParentDriver
type ChildContext struct {
	// PID of the child, can be used for ns-entering to the child namespaces.
	PID int
	// IP of the tap device
	IP net.IP
}

// ParentDriver is a driver for the parent process.
type ParentDriver interface {
	Manager
	// OpaqueForChild typically consists of socket path
	// for controlling child from parent
	OpaqueForChild() map[string]string
	// RunParentDriver signals initComplete when ParentDriver is ready to
	// serve as Manager.
	// RunParentDriver blocks until quit is signaled.
	//
	// ChildContext is optional.
	RunParentDriver(initComplete chan struct{}, quit <-chan struct{}, cctx *ChildContext) error
}

type ChildDriver interface {
	RunChildDriver(opaque map[string]string, quit <-chan struct{}) error
}
