//go:build windows

package hypervctl

import (
	"encoding/json"
	"fmt"

	"github.com/containers/libhvee/pkg/powershell"
)

// DriveInfo represents a drive attached to the SCSI controller
type DriveInfo struct {
	Path                          string `json:"Path,omitempty"`
	DiskNumber                    *int   `json:"DiskNumber,omitempty"`
	MaximumIOPS                   uint64 `json:"MaximumIOPS,omitempty"`
	MinimumIOPS                   uint64 `json:"MinimumIOPS,omitempty"`
	QoSPolicyID                   string `json:"QoSPolicyID,omitempty"`
	SupportPersistentReservations bool   `json:"SupportPersistentReservations,omitempty"`
	WriteHardeningMethod          int    `json:"WriteHardeningMethod,omitempty"`
	ControllerLocation            int    `json:"ControllerLocation,omitempty"`
	ControllerNumber              int    `json:"ControllerNumber,omitempty"`
	ControllerType                int    `json:"ControllerType,omitempty"`
	Name                          string `json:"Name,omitempty"`
	PoolName                      string `json:"PoolName,omitempty"`
	Id                            string `json:"Id,omitempty"`
	VMId                          string `json:"VMId,omitempty"`
	VMName                        string `json:"VMName,omitempty"`
	VMSnapshotId                  string `json:"VMSnapshotId,omitempty"`
	VMSnapshotName                string `json:"VMSnapshotName,omitempty"`
	ComputerName                  string `json:"ComputerName,omitempty"`
	IsDeleted                     bool   `json:"IsDeleted,omitempty"`
	DvdMediaType                  *int   `json:"DvdMediaType,omitempty"` // Only for DVD drives
}

type ScsiControllerSettings struct {
	// VMName specifies the name of the virtual machine to which the SCSI controller
	// is to be added. This is a mandatory parameter in one of the parameter sets.
	VMName string `json:"VMName,omitempty"`

	// Passthru specifies that a Microsoft.HyperV.PowerShell.VMScsiController object
	// representing the new SCSI controller should be passed through to the pipeline.
	Passthru bool `json:"passthru,omitempty"`

	// ComputerName specifies one or more Hyper-V hosts on which the SCSI controller
	// should be added.
	ComputerName string `json:"ComputerName,omitempty"`

	Confirm bool `json:"confirm,omitempty"`

	// Fields populated by Update method from Get-VMScsiController JSON output
	ControllerNumber int         `json:"ControllerNumber,omitempty"`
	IsTemplate       bool        `json:"IsTemplate,omitempty"`
	Drives           []DriveInfo `json:"Drives,omitempty"`
	Name             string      `json:"Name,omitempty"`
	Id               string      `json:"Id,omitempty"`
	VMId             string      `json:"VMId,omitempty"`
	VMSnapshotId     string      `json:"VMSnapshotId,omitempty"`
	VMSnapshotName   string      `json:"VMSnapshotName,omitempty"`
	IsDeleted        bool        `json:"IsDeleted,omitempty"`
	VMCheckpointId   string      `json:"VMCheckpointId,omitempty"`
	VMCheckpointName string      `json:"VMCheckpointName,omitempty"`
}

func (c *ScsiControllerSettings) GenerateAddCommand() []string {
	return []string{
		"Hyper-V\\Add-VMScsiController",
		"-VMName", c.VMName,
	}
}

func (c *ScsiControllerSettings) Update() (*ScsiControllerSettings, error) {
	if c.VMName == "" {
		return nil, fmt.Errorf("VM name cannot be empty")
	}

	// Execute PowerShell command to get SCSI controller info as JSON
	cmd := fmt.Sprintf("Get-VMScsiController -VMName '%s' | ConvertTo-Json", c.VMName)
	stdout, stderr, err := powershell.Execute(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to execute PowerShell command: %w, stderr: %s", err, stderr)
	}

	// Parse JSON response
	var controllerData interface{}
	if err := json.Unmarshal([]byte(stdout), &controllerData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON output: %w", err)
	}

	// Handle both single controller and array of controllers
	var controllers []map[string]interface{}
	switch data := controllerData.(type) {
	case map[string]interface{}:
		// Single controller
		controllers = []map[string]interface{}{data}
	case []interface{}:
		// Multiple controllers - convert to map slice
		for _, ctrl := range data {
			if ctrlMap, ok := ctrl.(map[string]interface{}); ok {
				controllers = append(controllers, ctrlMap)
			}
		}
	default:
		return nil, fmt.Errorf("unexpected JSON structure")
	}

	// Find the first controller (or by specific controller number if needed)
	if len(controllers) == 0 {
		return nil, fmt.Errorf("no SCSI controllers found for VM: %s", c.VMName)
	}

	// Use the first controller for now
	controllerMap := controllers[0]

	// Parse the controller data
	if err := c.parseControllerData(controllerMap); err != nil {
		return nil, fmt.Errorf("failed to parse controller data: %w", err)
	}

	return c, nil
}

// parseControllerData maps the JSON data from Get-VMScsiController to the struct fields
func (c *ScsiControllerSettings) parseControllerData(data map[string]interface{}) error {
	// Parse controller-level fields
	if val, ok := data["ControllerNumber"].(float64); ok {
		c.ControllerNumber = int(val)
	}

	if val, ok := data["IsTemplate"].(bool); ok {
		c.IsTemplate = val
	}

	if val, ok := data["Name"].(string); ok {
		c.Name = val
	}

	if val, ok := data["Id"].(string); ok {
		c.Id = val
	}

	if val, ok := data["VMId"].(string); ok {
		c.VMId = val
	}

	if val, ok := data["VMName"].(string); ok {
		c.VMName = val
	}

	if val, ok := data["VMSnapshotId"].(string); ok {
		c.VMSnapshotId = val
	}

	if val, ok := data["VMSnapshotName"].(string); ok {
		c.VMSnapshotName = val
	}

	if val, ok := data["ComputerName"].(string); ok {
		c.ComputerName = val
	}

	if val, ok := data["IsDeleted"].(bool); ok {
		c.IsDeleted = val
	}

	if val, ok := data["VMCheckpointId"].(string); ok {
		c.VMCheckpointId = val
	}

	if val, ok := data["VMCheckpointName"].(string); ok {
		c.VMCheckpointName = val
	}

	// Parse drives array
	if drivesData, ok := data["Drives"].([]interface{}); ok {
		c.Drives = make([]DriveInfo, 0, len(drivesData))
		for _, driveInterface := range drivesData {
			if driveMap, ok := driveInterface.(map[string]interface{}); ok {
				drive := DriveInfo{}

				if val, ok := driveMap["Path"].(string); ok {
					drive.Path = val
				}

				// DiskNumber can be null
				if val, ok := driveMap["DiskNumber"].(float64); ok {
					diskNum := int(val)
					drive.DiskNumber = &diskNum
				}

				if val, ok := driveMap["MaximumIOPS"].(float64); ok {
					drive.MaximumIOPS = uint64(val)
				}

				if val, ok := driveMap["MinimumIOPS"].(float64); ok {
					drive.MinimumIOPS = uint64(val)
				}

				if val, ok := driveMap["QoSPolicyID"].(string); ok {
					drive.QoSPolicyID = val
				}

				if val, ok := driveMap["SupportPersistentReservations"].(bool); ok {
					drive.SupportPersistentReservations = val
				}

				if val, ok := driveMap["WriteHardeningMethod"].(float64); ok {
					drive.WriteHardeningMethod = int(val)
				}

				if val, ok := driveMap["ControllerLocation"].(float64); ok {
					drive.ControllerLocation = int(val)
				}

				if val, ok := driveMap["ControllerNumber"].(float64); ok {
					drive.ControllerNumber = int(val)
				}

				if val, ok := driveMap["ControllerType"].(float64); ok {
					drive.ControllerType = int(val)
				}

				if val, ok := driveMap["Name"].(string); ok {
					drive.Name = val
				}

				if val, ok := driveMap["PoolName"].(string); ok {
					drive.PoolName = val
				}

				if val, ok := driveMap["Id"].(string); ok {
					drive.Id = val
				}

				if val, ok := driveMap["VMId"].(string); ok {
					drive.VMId = val
				}

				if val, ok := driveMap["VMName"].(string); ok {
					drive.VMName = val
				}

				if val, ok := driveMap["VMSnapshotId"].(string); ok {
					drive.VMSnapshotId = val
				}

				if val, ok := driveMap["VMSnapshotName"].(string); ok {
					drive.VMSnapshotName = val
				}

				if val, ok := driveMap["ComputerName"].(string); ok {
					drive.ComputerName = val
				}

				if val, ok := driveMap["IsDeleted"].(bool); ok {
					drive.IsDeleted = val
				}

				// DvdMediaType is only present for DVD drives
				if val, ok := driveMap["DvdMediaType"].(float64); ok {
					mediaType := int(val)
					drive.DvdMediaType = &mediaType
				}

				c.Drives = append(c.Drives, drive)
			}
		}
	}

	return nil
}

func (c *ScsiControllerSettings) AddSyntheticDiskDrive(slot uint) (*SyntheticDiskDriveSettings, error) {
	drive := &SyntheticDiskDriveSettings{}
	drive.DiskNumber = int(slot)
	drive.VMName = c.VMName

	drive.controllerSettings = c
	return drive, nil
}

func (c *ScsiControllerSettings) AddSyntheticDvdDrive(slot uint) (*SyntheticDvdDriveSettings, error) {
	drive := &SyntheticDvdDriveSettings{}
	drive.VMName = c.VMName
	drive.ControllerLocation = int(slot)
	drive.ControllerNumber = c.ControllerNumber

	drive.controllerSettings = c
	return drive, nil
}
