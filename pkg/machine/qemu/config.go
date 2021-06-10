// +build amd64,linux arm64,linux amd64,darwin arm64,darwin

package qemu

import "time"

type MachineVM struct {
	// CPUs to be assigned to the VM
	CPUs uint64
	// The command line representation of the qemu command
	CmdLine []string
	// IdentityPath is the fq path to the ssh priv key
	IdentityPath string
	// IgnitionFilePath is the fq path to the .ign file
	IgnitionFilePath string
	// ImagePath is the fq path to
	ImagePath string
	// Memory in megabytes assigned to the vm
	Memory uint64
	// Name of the vm
	Name string
	// SSH port for user networking
	Port int
	// QMPMonitor is the qemu monitor object for sending commands
	QMPMonitor Monitor
	// RemoteUsername of the vm user
	RemoteUsername string
}

type Monitor struct {
	//	Address portion of the qmp monitor (/tmp/tmp.sock)
	Address string
	// Network portion of the qmp monitor (unix)
	Network string
	// Timeout in seconds for qmp monitor transactions
	Timeout time.Duration
}

var (
	// defaultQMPTimeout is the timeout duration for the
	// qmp monitor interactions
	defaultQMPTimeout time.Duration = 2 * time.Second
	// defaultRemoteUser describes the ssh username default
	defaultRemoteUser = "core"
)
