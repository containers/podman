//go:build (amd64 && !windows) || (arm64 && !windows)
// +build amd64,!windows arm64,!windows

package qemu

import (
	"time"

	"github.com/containers/podman/v4/pkg/machine"
)

const (
	// FCOS streams
	// Testing FCOS stream
	Testing string = "testing"
	// Next FCOS stream
	Next string = "next"
	// Stable FCOS stream
	Stable string = "stable"

	// Max length of fully qualified socket path

)

type Provider struct{}

// Deprecated: MachineVMV1 is being deprecated in favor a more flexible and informative
// structure
type MachineVMV1 struct {
	// CPUs to be assigned to the VM
	CPUs uint64
	// The command line representation of the qemu command
	CmdLine []string
	// Mounts is the list of remote filesystems to mount
	Mounts []machine.Mount
	// IdentityPath is the fq path to the ssh priv key
	IdentityPath string
	// IgnitionFilePath is the fq path to the .ign file
	IgnitionFilePath string
	// ImageStream is the update stream for the image
	ImageStream string
	// ImagePath is the fq path to
	ImagePath string
	// Memory in megabytes assigned to the vm
	Memory uint64
	// Disk size in gigabytes assigned to the vm
	DiskSize uint64
	// Name of the vm
	Name string
	// SSH port for user networking
	Port int
	// QMPMonitor is the qemu monitor object for sending commands
	QMPMonitor Monitorv1
	// RemoteUsername of the vm user
	RemoteUsername string
	// Whether this machine should run in a rootful or rootless manner
	Rootful bool
	// UID is the numerical id of the user that called machine
	UID int
}

type MachineVM struct {
	// ConfigPath is the path to the configuration file
	ConfigPath machine.VMFile
	// The command line representation of the qemu command
	CmdLine []string
	// HostUser contains info about host user
	machine.HostUser
	// ImageConfig describes the bootable image
	machine.ImageConfig
	// Mounts is the list of remote filesystems to mount
	Mounts []machine.Mount
	// Name of VM
	Name string
	// PidFilePath is the where the Proxy PID file lives
	PidFilePath machine.VMFile
	// VMPidFilePath is the where the VM PID file lives
	VMPidFilePath machine.VMFile
	// QMPMonitor is the qemu monitor object for sending commands
	QMPMonitor Monitor
	// ReadySocket tells host when vm is booted
	ReadySocket machine.VMFile
	// ResourceConfig is physical attrs of the VM
	machine.ResourceConfig
	// SSHConfig for accessing the remote vm
	machine.SSHConfig
	// Starting tells us whether the machine is running or if we have just dialed it to start it
	Starting bool
	// Created contains the original created time instead of querying the file mod time
	Created time.Time
	// LastUp contains the last recorded uptime
	LastUp time.Time
}

type Monitorv1 struct {
	//	Address portion of the qmp monitor (/tmp/tmp.sock)
	Address string
	// Network portion of the qmp monitor (unix)
	Network string
	// Timeout in seconds for qmp monitor transactions
	Timeout time.Duration
}

type Monitor struct {
	//	Address portion of the qmp monitor (/tmp/tmp.sock)
	Address machine.VMFile
	// Network portion of the qmp monitor (unix)
	Network string
	// Timeout in seconds for qmp monitor transactions
	Timeout time.Duration
}

var (
	// defaultQMPTimeout is the timeout duration for the
	// qmp monitor interactions.
	defaultQMPTimeout = 2 * time.Second
)
