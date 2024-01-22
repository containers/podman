package vmconfigs

import (
	"errors"
	"net/url"
	"time"

	gvproxy "github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/containers/podman/v4/pkg/machine/define"
	"github.com/containers/podman/v4/pkg/machine/qemu/command"
	"github.com/containers/podman/v4/pkg/strongunits"
	"github.com/containers/storage/pkg/lockfile"
)

type MachineConfig struct {
	// Common stuff
	Created  time.Time
	GvProxy  gvproxy.GvproxyCommand
	HostUser HostUser

	LastUp time.Time

	Mounts []Mount
	Name   string

	Resources ResourceConfig
	SSH       SSHConfig
	Version   uint

	// Image stuff
	imageDescription machineImage //nolint:unused

	ImagePath *define.VMFile // Temporary only until a proper image struct is worked out

	// Provider stuff
	AppleHypervisor  *AppleHVConfig `json:",omitempty"`
	QEMUHypervisor   *QEMUConfig    `json:",omitempty"`
	HyperVHypervisor *HyperVConfig  `json:",omitempty"`
	WSLHypervisor    *WSLConfig     `json:",omitempty"`

	lock *lockfile.LockFile //nolint:unused

	// configPath can be used for reading, writing, removing
	configPath *define.VMFile

	// used for deriving file, socket, etc locations
	dirs *define.MachineDirs

	// State

	// Starting is defined as "on" but not fully booted
	Starting bool
}

// MachineImage describes a podman machine image
type MachineImage struct {
	OCI  *OCIMachineImage
	FCOS *fcosMachineImage
}

// Pull downloads a machine image
func (m *MachineImage) Pull() error {
	if m.OCI != nil {
		return m.OCI.download()
	}
	if m.FCOS != nil {
		return m.FCOS.download()
	}
	return errors.New("no valid machine image provider detected")
}

type machineImage interface { //nolint:unused
	download() error
	path() string
}

type OCIMachineImage struct {
	// registry
	// TODO JSON serial/deserial will write string to disk
	// but in code it is a types.ImageReference

	// quay.io/podman/podman-machine-image:5.0
	FQImageReference string
}

func (o OCIMachineImage) path() string {
	return ""
}

func (o OCIMachineImage) download() error {
	return nil
}

type fcosMachineImage struct {
	// TODO JSON serial/deserial will write string to disk
	// but in code is url.URL
	Location url.URL // file://path/.qcow2  https://path/qcow2
}

func (f fcosMachineImage) download() error {
	return nil
}

func (f fcosMachineImage) path() string {
	return ""
}

type VMProvider interface { //nolint:interfacebloat
	CreateVM(opts define.CreateVMOpts, mc *MachineConfig) error
	GetHyperVisorVMs() ([]string, error)
	MountType() VolumeMountType
	MountVolumesToVM(mc *MachineConfig, quiet bool) error
	Remove(mc *MachineConfig) ([]string, func() error, error)
	RemoveAndCleanMachines(dirs *define.MachineDirs) error
	SetProviderAttrs(mc *MachineConfig, cpus, memory *uint64, newDiskSize *strongunits.GiB) error
	StartNetworking(mc *MachineConfig, cmd *gvproxy.GvproxyCommand) error
	StartVM(mc *MachineConfig) (func() error, func() error, error)
	State(mc *MachineConfig, bypass bool) (define.Status, error)
	StopVM(mc *MachineConfig, hardStop bool) error
	StopHostNetworking() error
	VMType() define.VMType
}

// HostUser describes the host user
type HostUser struct {
	// Whether this machine should run in a rootful or rootless manner
	Rootful bool
	// UID is the numerical id of the user that called machine
	UID int
	// Whether one of these fields has changed and actions should be taken
	Modified bool `json:"HostUserModified"`
}

type Mount struct {
	ReadOnly      bool
	Source        string
	Tag           string
	Target        string
	Type          string
	OriginalInput string
}

// ResourceConfig describes physical attributes of the machine
type ResourceConfig struct {
	// CPUs to be assigned to the VM
	CPUs uint64
	// Disk size in gigabytes assigned to the vm
	DiskSize uint64
	// Memory in megabytes assigned to the vm
	Memory uint64
	// Usbs
	USBs []command.USBConfig
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

type VMStats struct {
	// Created contains the original created time instead of querying the file mod time
	Created time.Time
	// LastUp contains the last recorded uptime
	LastUp time.Time
}
