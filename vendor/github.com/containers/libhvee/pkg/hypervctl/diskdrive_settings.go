//go:build windows

package hypervctl

import (
	"fmt"

	"github.com/containers/libhvee/pkg/powershell"
)

// HardDiskDriveSettings represents the arguments for the PowerShell cmdlet
// "Add-VMHardDiskDrive". This type can be used to add a new virtual hard disk
// to a Hyper-V virtual machine.
//
// Pointers are used for optional fields to differentiate between a zero value
// and a field that was not explicitly provided.
type HardDiskDriveSettings struct {
	// VMName specifies the name of the virtual machine to which the hard disk
	// is to be added.
	VMName string `json:"VMName,omitempty"`

	// ControllerType specifies the type of controller (IDE or SCSI) for the hard disk.
	ControllerType string `json:"ControllerType,omitempty"`

	// ControllerNumber specifies the number of the controller.
	ControllerNumber int `json:"ControllerNumber,omitempty"`

	// ControllerLocation specifies the location number on the controller.
	ControllerLocation int `json:"ControllerLocation,omitempty"`

	// Path specifies the full path of the hard disk drive file to be added.
	Path string `json:"Path,omitempty"`

	// DiskNumber specifies the disk number of an offline physical hard drive
	// to be connected as a passthrough disk.
	DiskNumber int `json:"DiskNumber,omitempty"`

	// AllowUnverifiedPaths specifies that no error should be thrown if the path
	// is not verified for clustered virtual machines.
	AllowUnverifiedPaths bool `json:"AllowUnverifiedPaths,omitempty"`

	// Passthru specifies that the added HardDiskDrive object should be
	// passed through to the pipeline.
	Passthru bool `json:"Passthru,omitempty"`

	// MaximumIOPS specifies the maximum normalized I/O operations per second (IOPS).
	MaximumIOPS uint64 `json:"MaximumIOPS,omitempty"`

	// MinimumIOPS specifies the minimum normalized I/O operations per second (IOPS).
	MinimumIOPS uint64 `json:"MinimumIOPS,omitempty"`

	// QoSPolicy specifies the name of a storage Quality of Service (QoS) policy.
	QoSPolicy string `json:"QoSPolicy,omitempty"`

	// QoSPolicyID specifies the unique ID for a storage QoS policy.
	QoSPolicyID string `json:"QoSPolicyID,omitempty"`

	// ResourcePoolName specifies the friendly name of the ISO resource pool.
	ResourcePoolName string `json:"ResourcePoolName,omitempty"`

	// SupportPersistentReservations indicates that the hard disk supports SCSI
	// persistent reservation semantics for shared disks.
	SupportPersistentReservations bool `json:"SupportPersistentReservations,omitempty"`

	// Other optional parameters for remote sessions or confirmation.
	ComputerName string `json:"ComputerName,omitempty"`
}

type SyntheticDiskDriveSettings struct {
	HardDiskDriveSettings
	controllerSettings *ScsiControllerSettings
}


func (d *SyntheticDiskDriveSettings) DefineVirtualHardDisk(vhdxFile string, beforeAdd func(*HardDiskDriveSettings)) (*HardDiskDriveSettings, error) {
	vhd := &HardDiskDriveSettings{}

	if beforeAdd != nil {
		beforeAdd(vhd)
	}
	vhd.Path = vhdxFile
	vhd.VMName = d.VMName
	vhd.ControllerType = d.ControllerType
	vhd.ControllerNumber = d.ControllerNumber
	vhd.ControllerLocation = d.ControllerLocation
	vhd.DiskNumber = d.DiskNumber
	vhd.MaximumIOPS = d.MaximumIOPS
	vhd.MinimumIOPS = d.MinimumIOPS

	cli := vhd.getCLI()
	cli = append([]string{"Hyper-V\\Add-VMHardDiskDrive"}, cli...)
	_, stderr, err := powershell.Execute(cli...)
	if err != nil {
		return nil, NewPSError(stderr)
	}

	return vhd, nil
}

// GetCLI generates PowerShell Add-VMHardDiskDrive command parameters from HardDiskDriveSettings
func (h *HardDiskDriveSettings) getCLI() []string {
	if h.VMName == "" {
		return []string{}
	}

	params := []string{}

	// VM Name is required
	params = append(params, "-VMName", fmt.Sprintf("'%s'", h.VMName))

	// String parameters
	if h.Path != "" {
		params = append(params, "-Path", fmt.Sprintf("'%s'", h.Path))
	}
	if h.ControllerType != "" {
		params = append(params, "-ControllerType", h.ControllerType)
	}
	if h.QoSPolicy != "" {
		params = append(params, "-QoSPolicy", fmt.Sprintf("'%s'", h.QoSPolicy))
	}
	if h.QoSPolicyID != "" {
		params = append(params, "-QoSPolicyID", fmt.Sprintf("'%s'", h.QoSPolicyID))
	}
	if h.ResourcePoolName != "" {
		params = append(params, "-ResourcePoolName", fmt.Sprintf("'%s'", h.ResourcePoolName))
	}
	if h.ComputerName != "" {
		params = append(params, "-ComputerName", fmt.Sprintf("'%s'", h.ComputerName))
	}

	// Numeric parameters (note: some parameters allow 0 as valid value in PowerShell)
	if h.ControllerNumber >= 0 {
		params = append(params, "-ControllerNumber", fmt.Sprintf("%d", h.ControllerNumber))
	}
	if h.ControllerLocation >= 0 {
		params = append(params, "-ControllerLocation", fmt.Sprintf("%d", h.ControllerLocation))
	}
	if h.DiskNumber > 0 {
		params = append(params, "-DiskNumber", fmt.Sprintf("%d", h.DiskNumber))
	}
	if h.MaximumIOPS > 0 {
		params = append(params, "-MaximumIOPS", fmt.Sprintf("%d", h.MaximumIOPS))
	}
	if h.MinimumIOPS > 0 {
		params = append(params, "-MinimumIOPS", fmt.Sprintf("%d", h.MinimumIOPS))
	}

	// Boolean parameters (switch parameters only need to be present for true values)
	if h.AllowUnverifiedPaths {
		params = append(params, "-AllowUnverifiedPaths")
	}
	if h.Passthru {
		params = append(params, "-Passthru")
	}
	if h.SupportPersistentReservations {
		params = append(params, "-SupportPersistentReservations")
	}

	return params
}
