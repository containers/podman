//go:build windows

package hypervctl

import (
	"fmt"

	"github.com/containers/libhvee/pkg/powershell"
)

// DvdDriveSettings represents the arguments for the PowerShell cmdlet
// "Add-VMDvdDrive". This type can be used to add a new DVD drive
// to a Hyper-V virtual machine.
//
// Pointers are used for optional fields to differentiate between a zero value
// and a field that was not explicitly provided.
type DvdDriveSettings struct {
	// VMName specifies the name of the virtual machine to which the DVD drive
	// is to be added. This is a mandatory parameter in one of the parameter sets.
	VMName string `json:"VMName,omitempty"`

	// ControllerNumber specifies the number of the controller.
	ControllerNumber int `json:"ControllerNumber,omitempty"`

	// ControllerLocation specifies the location number on the controller.
	ControllerLocation int `json:"ControllerLocation,omitempty"`

	// Path specifies the full path to the virtual DVD media file (.iso).
	Path string `json:"Path,omitempty"`

	// AllowUnverifiedPaths specifies that no error should be thrown if the path
	// is not verified for clustered virtual machines.
	AllowUnverifiedPaths bool `json:"AllowUnverifiedPaths,omitempty"`

	// Passthru specifies that the added DvdDrive object should be
	// passed through to the pipeline.
	Passthru bool `json:"Passthru,omitempty"`

	// ResourcePoolName specifies the friendly name of the ISO resource pool.
	ResourcePoolName string `json:"ResourcePoolName,omitempty"`

	// Other optional parameters for remote sessions or confirmation.
	ComputerName string `json:"ComputerName,omitempty"`
	Confirm      bool   `json:"Confirm,omitempty"`
}

// GetCLI generates PowerShell Add-VMDvdDrive command parameters from DvdDriveSettings
func (d *DvdDriveSettings) GetCLI() []string {
	if d.VMName == "" {
		return []string{}
	}

	params := []string{}

	// VM Name is required
	params = append(params, "-VMName", fmt.Sprintf("'%s'", d.VMName))

	// String parameters
	if d.Path != "" {
		params = append(params, "-Path", fmt.Sprintf("'%s'", d.Path))
	}
	if d.ResourcePoolName != "" {
		params = append(params, "-ResourcePoolName", fmt.Sprintf("'%s'", d.ResourcePoolName))
	}
	if d.ComputerName != "" {
		params = append(params, "-ComputerName", fmt.Sprintf("'%s'", d.ComputerName))
	}

	// Numeric parameters (note: some parameters allow 0 as valid value in PowerShell)
	if d.ControllerNumber >= 0 {
		params = append(params, "-ControllerNumber", fmt.Sprintf("%d", d.ControllerNumber))
	}
	if d.ControllerLocation >= 0 {
		params = append(params, "-ControllerLocation", fmt.Sprintf("%d", d.ControllerLocation))
	}

	// Boolean parameters (switch parameters only need to be present for true values)
	if d.AllowUnverifiedPaths {
		params = append(params, "-AllowUnverifiedPaths")
	}
	if d.Passthru {
		params = append(params, "-Passthru")
	}
	if d.Confirm {
		params = append(params, "-Confirm")
	}

	return params
}

type SyntheticDvdDriveSettings struct {
	DvdDriveSettings
	controllerSettings *ScsiControllerSettings
}

func (d *SyntheticDvdDriveSettings) DefineVirtualDvdDisk(imageFile string) (*DvdDriveSettings, error) {
	dvd := &DvdDriveSettings{}

	// Copy settings from parent
	dvd.VMName = d.VMName
	dvd.ControllerNumber = d.ControllerNumber
	dvd.ControllerLocation = d.ControllerLocation
	dvd.Path = imageFile
	dvd.AllowUnverifiedPaths = d.AllowUnverifiedPaths
	dvd.Passthru = d.Passthru
	dvd.ResourcePoolName = d.ResourcePoolName
	dvd.ComputerName = d.ComputerName
	dvd.Confirm = d.Confirm

	// Generate CLI command and execute
	cli := dvd.GetCLI()
	cli = append([]string{"Hyper-V\\Add-VMDvdDrive"}, cli...)
	_, stderr, err := powershell.Execute(cli...)
	if err != nil {
		return nil, NewPSError(stderr)
	}

	return dvd, nil
}
