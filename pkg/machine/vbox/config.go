//go:build (amd64 && !windows) || !arm64
// +build amd64,!windows !arm64

package vbox

import "time"

type Provider struct{}

type MachineVM struct {
	// Name of the vm
	Name string
	// VBox option ostype
	VBOSType string
	// CPUs to be assigned to the VM
	CPUs uint64
	// Memory in megabytes assigned to the vm
	Memory uint64
	// Disk size in gigabytes assigned to the vm
	DiskSize uint64
	// The fq path to VDI file
	VIDPath string
	// ImagePath is the fq path to
	ImagePath string
	// ImageStream is the update stream for the image
	ImageStream string
	// BaseFolder folder where VBox store .vbox files
	BaseFolder string
	// IgnitionFilePath is the fq path to the .ign file
	IgnitionFilePath string
	// SSH port for user networking
	Port int
	// RemoteUsername of the vm user
	RemoteUsername string
	// IdentityPath is the fq path to the ssh priv key
	IdentityPath string
	// Path to VBoxManage
	VBoxManageExecPath string
	// Whether this machine should run in a rootful or rootless manner
	Rootful bool
	// Time when VM has been created
	CreatingTime time.Time
}

// For future usage
type FcosArtifact struct {
	Artifact string
	Format   string
	Stream   string
}
