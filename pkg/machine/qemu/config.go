//go:build (amd64 && !windows) || (arm64 && !windows)
// +build amd64,!windows arm64,!windows

package qemu

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
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
	maxSocketPathLength int = 103
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
	Mounts []Mount
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
	ConfigPath MachineFile
	// The command line representation of the qemu command
	CmdLine []string
	// HostUser contains info about host user
	HostUser
	// ImageConfig describes the bootable image
	ImageConfig
	// Mounts is the list of remote filesystems to mount
	Mounts []Mount
	// Name of VM
	Name string
	// PidFilePath is the where the PID file lives
	PidFilePath MachineFile
	// QMPMonitor is the qemu monitor object for sending commands
	QMPMonitor Monitor
	// ReadySocket tells host when vm is booted
	ReadySocket MachineFile
	// ResourceConfig is physical attrs of the VM
	ResourceConfig
	// SSHConfig for accessing the remote vm
	SSHConfig
	// Starting tells us whether the machine is running or if we have just dialed it to start it
	Starting bool
	// Created contains the original created time instead of querying the file mod time
	Created time.Time
	// LastUp contains the last recorded uptime
	LastUp time.Time
}

// ImageConfig describes the bootable image for the VM
type ImageConfig struct {
	IgnitionFilePath MachineFile
	// ImageStream is the update stream for the image
	ImageStream string
	// ImagePath is the fq path to
	ImagePath MachineFile
}

// HostUser describes the host user
type HostUser struct {
	// Whether this machine should run in a rootful or rootless manner
	Rootful bool
	// UID is the numerical id of the user that called machine
	UID int
}

// SSHConfig contains remote access information for SSH
type SSHConfig struct {
	// IdentityPath is the fq path to the ssh priv key
	IdentityPath string
	// SSH port for user networking
	Port int
	// RemoteUsername of the vm user
	RemoteUsername string
}

// ResourceConfig describes physical attributes of the machine
type ResourceConfig struct {
	// CPUs to be assigned to the VM
	CPUs uint64
	// Memory in megabytes assigned to the vm
	Memory uint64
	// Disk size in gigabytes assigned to the vm
	DiskSize uint64
}

type MachineFile struct {
	// Path is the fully qualified path to a file
	Path string
	// Symlink is a shortened version of Path by using
	// a symlink
	Symlink *string
}

type Mount struct {
	Type     string
	Tag      string
	Source   string
	Target   string
	ReadOnly bool
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
	Address MachineFile
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

// GetPath returns the working path for a machinefile.  it returns
// the symlink unless one does not exist
func (m *MachineFile) GetPath() string {
	if m.Symlink == nil {
		return m.Path
	}
	return *m.Symlink
}

// Delete removes the machinefile symlink (if it exists) and
// the actual path
func (m *MachineFile) Delete() error {
	if m.Symlink != nil {
		if err := os.Remove(*m.Symlink); err != nil && !errors.Is(err, os.ErrNotExist) {
			logrus.Errorf("unable to remove symlink %q", *m.Symlink)
		}
	}
	if err := os.Remove(m.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Read the contents of a given file and return in []bytes
func (m *MachineFile) Read() ([]byte, error) {
	return ioutil.ReadFile(m.GetPath())
}

// NewMachineFile is a constructor for MachineFile
func NewMachineFile(path string, symlink *string) (*MachineFile, error) {
	if len(path) < 1 {
		return nil, errors.New("invalid machine file path")
	}
	if symlink != nil && len(*symlink) < 1 {
		return nil, errors.New("invalid symlink path")
	}
	mf := MachineFile{Path: path}
	if symlink != nil && len(path) > maxSocketPathLength {
		if err := mf.makeSymlink(symlink); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	return &mf, nil
}

// makeSymlink for macOS creates a symlink in $HOME/.podman/
// for a machinefile like a socket
func (m *MachineFile) makeSymlink(symlink *string) error {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	sl := filepath.Join(homedir, ".podman", *symlink)
	// make the symlink dir and throw away if it already exists
	if err := os.MkdirAll(filepath.Dir(sl), 0700); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	m.Symlink = &sl
	return os.Symlink(m.Path, sl)
}
