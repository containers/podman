//go:build linux
// +build linux

// Package rootlessport provides reexec for RootlessKit-based port forwarder.
//
// init() contains reexec.Register() for ReexecKey .
//
// The reexec requires Config to be provided via stdin.
//
// The reexec writes human-readable error message on stdout on error.
//
// Debug log is printed on stderr.
package rootlessport

import (
	"github.com/containers/common/libnetwork/types"
)

const (
	// BinaryName is the binary name for the parent process.
	BinaryName = "rootlessport"
)

// Config needs to be provided to the process via stdin as a JSON string.
// stdin needs to be closed after the message has been written.
type Config struct {
	Mappings    []types.PortMapping
	NetNSPath   string
	ExitFD      int
	ReadyFD     int
	TmpDir      string
	ChildIP     string
	ContainerID string
	RootlessCNI bool
}
