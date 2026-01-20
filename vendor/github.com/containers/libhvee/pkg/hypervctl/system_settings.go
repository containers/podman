//go:build windows

package hypervctl

import (
	"encoding/json"
	"fmt"

	"github.com/containers/libhvee/pkg/powershell"
)

// AutomaticStartActionType defines the possible actions for a VM when the Hyper-V host starts.
type AutomaticStartActionType string

const (
	AutomaticStartActionNothing        AutomaticStartActionType = "Nothing"
	AutomaticStartActionStart          AutomaticStartActionType = "Start"
	AutomaticStartActionStartIfRunning AutomaticStartActionType = "StartIfRunning"
	AutomaticStartActionNone           AutomaticStartActionType = "None"
)

// AutomaticStopActionType defines the possible actions for a VM when the Hyper-V host stops.
type AutomaticStopActionType string

const (
	AutomaticStopActionShutDown AutomaticStopActionType = "ShutDown"
	AutomaticStopActionSave     AutomaticStopActionType = "Save"
	AutomaticStopActionTurnOff  AutomaticStopActionType = "TurnOff"
	AutomaticStopActionNone     AutomaticStopActionType = "None"
)

// AutomaticCriticalErrorActionType defines the possible actions for a VM when a critical error occurs.
type AutomaticCriticalErrorActionType string

const (
	AutomaticCriticalErrorActionPause AutomaticCriticalErrorActionType = "Pause"
	AutomaticCriticalErrorActionNone  AutomaticCriticalErrorActionType = "None"
)

// CheckpointType defines the types of checkpoints to use for a virtual machine.
type CheckpointType string

const (
	CheckpointTypeDisabled       CheckpointType = "Disabled"
	CheckpointTypeStandard       CheckpointType = "Standard"
	CheckpointTypeProduction     CheckpointType = "Production"
	CheckpointTypeProductionOnly CheckpointType = "ProductionOnly"
	CheckpointTypeNone           CheckpointType = "None"
)

type NetworkBootProtocol string

const (
	IPv4 NetworkBootProtocol = "IPv4"
	IPv6 NetworkBootProtocol = "IPv6"
)

// SystemSettings represents the parameters available for the Set-VM PowerShell cmdlet.
// Each field corresponds to a parameter, using pointer types to indicate optionality
// (nil means the parameter is not set, non-nil means it is set).
type SystemSettings struct {
	// Name specifies the name of the virtual machine to configure. This is a mandatory parameter
	// for the PowerShell cmdlet, but here it's treated as optional for flexibility in a Go API.
	Name string

	// NewVMName specifies a new name for the virtual machine.
	NewVMName string

	// ProcessorCount specifies the number of virtual processors for the virtual machine.
	ProcessorCount uint64

	// DynamicMemory indicates whether dynamic memory is enabled for the virtual machine.
	DynamicMemory bool
	// StaticMemory indicates whether static memory is enabled for the virtual machine.
	StaticMemory bool

	// MemoryMinimumBytes specifies the minimum amount of memory, in bytes, that can be allocated to the virtual machine
	// when DynamicMemory is enabled.
	MemoryMinimumBytes uint64
	// MemoryMaximumBytes specifies the maximum amount of memory, in bytes, that can be allocated to the virtual machine
	// when DynamicMemory is enabled.
	MemoryMaximumBytes uint64
	// MemoryStartupBytes specifies the amount of memory, in bytes, that is allocated to the virtual machine when it starts.
	MemoryStartupBytes uint64

	// AutomaticStartAction specifies the action to take when the Hyper-V host starts.
	AutomaticStartAction AutomaticStartActionType
	// AutomaticStartDelay specifies the delay, in seconds, before the virtual machine automatically starts.
	AutomaticStartDelay uint32

	// AutomaticStopAction specifies the action to take when the Hyper-V host stops.
	AutomaticStopAction AutomaticStopActionType

	// AutomaticCriticalErrorAction specifies the action to take when a critical error occurs.
	AutomaticCriticalErrorAction AutomaticCriticalErrorActionType
	// AutomaticCriticalErrorActionTimeout specifies the timeout, in seconds, for the critical error action.
	AutomaticCriticalErrorActionTimeout uint32

	// AutomaticCheckpointsEnabled indicates whether automatic checkpoints are enabled for the virtual machine.
	AutomaticCheckpointsEnabled bool
	// CheckpointType specifies the type of checkpoints to use for the virtual machine.
	CheckpointType CheckpointType

	// SmartPagingFilePath specifies the path to the smart paging file.
	SmartPagingFilePath string
	// SnapshotFileLocation specifies the location where snapshot files are stored.
	SnapshotFileLocation string

	// AllowUnverifiedPaths indicates whether to allow unverified paths for clustered virtual machines.
	AllowUnverifiedPaths bool

	// GuestControlledCacheTypes indicates whether the guest operating system can control cache types.
	GuestControlledCacheTypes bool

	// HighMemoryMappedIoSpace specifies the amount of high memory mapped I/O space, in bytes.
	HighMemoryMappedIoSpace uint64
	// LowMemoryMappedIoSpace specifies the amount of low memory mapped I/O space, in bytes.
	LowMemoryMappedIoSpace uint64

	// LockOnDisconnect indicates whether the virtual machine should be locked on disconnect.
	LockOnDisconnect bool

	// Notes specifies a descriptive note for the virtual machine.
	Notes string

	// Passthru indicates that the cmdlet should return the virtual machine object after the operation.
	// This is a common PowerShell parameter, not directly a VM configuration.
	Passthru bool

	NetworkBootPreferredProtocol NetworkBootProtocol
}

func DefaultSystemSettings() *SystemSettings {
	return &SystemSettings{
		// setup all non-zero settings
		AutomaticStartAction:         AutomaticStartActionNothing,      // no auto-start
		AutomaticStopAction:          AutomaticStopActionShutDown,      // shutdown
		AutomaticCriticalErrorAction: AutomaticCriticalErrorActionNone, // restart
		CheckpointType:               CheckpointTypeDisabled,           // no snapshotting
		NetworkBootPreferredProtocol: IPv4,                             // ipv4 for pxe
	}

}

func (s *SystemSettings) AddScsiController() (*ScsiControllerSettings, error) {

	controller := &ScsiControllerSettings{}

	controller.VMName = s.Name

	cli := controller.GenerateAddCommand()

	_, stderr, err := powershell.Execute(cli...)
	if err != nil {
		return nil, fmt.Errorf("failed to add SCSI controller: %w", NewPSError(stderr))
	}

	updatedController, err := controller.Update()
	if err != nil {
		return nil, fmt.Errorf("failed to update SCSI controller: %w", err)
	}

	return updatedController, nil
}

func (s *SystemSettings) GetVM() (*SystemSettings, error) {
	return GetVMFromName(s.Name)
}

func getCLI(opts *SystemSettings) []string {
	if opts.Name == "" {
		return []string{}
	}

	params := []string{}

	// VM Name is required
	params = append(params, fmt.Sprintf("-Name '%s'", opts.Name))

	// String parameters
	if opts.NewVMName != "" {
		params = append(params, fmt.Sprintf("-NewVMName '%s'", opts.NewVMName))
	}
	if opts.SmartPagingFilePath != "" {
		params = append(params, fmt.Sprintf("-SmartPagingFilePath '%s'", opts.SmartPagingFilePath))
	}
	if opts.SnapshotFileLocation != "" {
		params = append(params, fmt.Sprintf("-SnapshotFileLocation '%s'", opts.SnapshotFileLocation))
	}
	if opts.Notes != "" {
		params = append(params, fmt.Sprintf("-Notes '%s'", opts.Notes))
	}

	// Numeric parameters
	if opts.ProcessorCount > 0 {
		params = append(params, fmt.Sprintf("-ProcessorCount %d", opts.ProcessorCount))
	}
	if opts.AutomaticStartDelay > 0 {
		params = append(params, fmt.Sprintf("-AutomaticStartDelay %d", opts.AutomaticStartDelay))
	}
	if opts.AutomaticCriticalErrorActionTimeout > 0 {
		params = append(params, fmt.Sprintf("-AutomaticCriticalErrorActionTimeout %d", opts.AutomaticCriticalErrorActionTimeout))
	}

	// Memory parameters (in bytes)
	if opts.MemoryStartupBytes > 0 {
		params = append(params, fmt.Sprintf("-MemoryStartupBytes %d", opts.MemoryStartupBytes))
	}
	if opts.MemoryMinimumBytes > 0 {
		params = append(params, fmt.Sprintf("-MemoryMinimumBytes %d", opts.MemoryMinimumBytes))
	}
	if opts.MemoryMaximumBytes > 0 {
		params = append(params, fmt.Sprintf("-MemoryMaximumBytes %d", opts.MemoryMaximumBytes))
	}
	if opts.HighMemoryMappedIoSpace > 0 {
		params = append(params, fmt.Sprintf("-HighMemoryMappedIoSpace %d", opts.HighMemoryMappedIoSpace))
	}
	if opts.LowMemoryMappedIoSpace > 0 {
		params = append(params, fmt.Sprintf("-LowMemoryMappedIoSpace %d", opts.LowMemoryMappedIoSpace))
	}

	// Boolean parameters
	if opts.DynamicMemory {
		params = append(params, "-DynamicMemory")
	}
	if opts.StaticMemory {
		params = append(params, "-StaticMemory")
	}
	if opts.AutomaticCheckpointsEnabled {
		params = append(params, "-AutomaticCheckpointsEnabled $True")
	}
	if opts.AllowUnverifiedPaths {
		params = append(params, "-AllowUnverifiedPaths")
	}
	if opts.LockOnDisconnect {
		params = append(params, "-LockOnDisconnect On")
	}
	if opts.Passthru {
		params = append(params, "-Passthru")
	}

	// Enum parameters
	//we dont need to set the None value
	if opts.AutomaticStartAction != "" && opts.AutomaticStartAction != AutomaticStartActionNone {
		params = append(params, fmt.Sprintf("-AutomaticStartAction %s", opts.AutomaticStartAction))
	}
	if opts.AutomaticStopAction != "" && opts.AutomaticStopAction != AutomaticStopActionNone {
		params = append(params, fmt.Sprintf("-AutomaticStopAction %s", opts.AutomaticStopAction))
	}
	if opts.AutomaticCriticalErrorAction != "" {
		params = append(params, fmt.Sprintf("-AutomaticCriticalErrorAction %s", opts.AutomaticCriticalErrorAction))
	}
	if opts.CheckpointType != "" && opts.CheckpointType != CheckpointTypeNone {
		params = append(params, fmt.Sprintf("-CheckpointType %s", opts.CheckpointType))
	}
	if opts.GuestControlledCacheTypes {
		params = append(params, "-GuestControlledCacheTypes $True")
	} else {
		params = append(params, "-GuestControlledCacheTypes $False")
	}

	return params
}

// GetVMFromName executes Get-VM PowerShell command and parses the JSON output into SystemSettings
func GetVMFromName(vmName string) (*SystemSettings, error) {
	if vmName == "" {
		return nil, fmt.Errorf("VM name cannot be empty")
	}

	// Execute PowerShell command to get VM info as JSON
	cmd := fmt.Sprintf("Get-VM -Name '%s' | ConvertTo-Json -Depth 5 -Compress", vmName)
	stdout, stderr, err := powershell.Execute(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to execute PowerShell command: %w, stderr: %s", err, stderr)
	}

	// Parse JSON response
	var vmData map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &vmData); err != nil {
		// hyperv allow to have multiple VMs with the same name
		// so we need to get the first one
		var vms []map[string]interface{}
		err := json.Unmarshal([]byte(stdout), &vms)
		if err != nil {
			return nil, fmt.Errorf("failed to parse JSON output: %w", err)
		}
		if len(vms) == 0 {
			return nil, fmt.Errorf("no VMs found with name: %s", vmName)
		}
		vmData = vms[0]
	}

	// Create SystemSettings instance and map fields
	settings := &SystemSettings{
		Name: vmName,
	}

	// Map PowerShell VM properties to SystemSettings fields
	if val, ok := vmData["ProcessorCount"].(float64); ok {
		settings.ProcessorCount = uint64(val)
	}

	if val, ok := vmData["DynamicMemoryEnabled"].(bool); ok {
		settings.DynamicMemory = val
		settings.StaticMemory = !val
	}

	if val, ok := vmData["MemoryMinimum"].(float64); ok {
		settings.MemoryMinimumBytes = uint64(val)
	}

	if val, ok := vmData["MemoryMaximum"].(float64); ok {
		settings.MemoryMaximumBytes = uint64(val)
	}

	if val, ok := vmData["MemoryStartup"].(float64); ok {
		settings.MemoryStartupBytes = uint64(val)
	}

	if val, ok := vmData["AutomaticStartAction"].(float64); ok {
		settings.AutomaticStartAction = cerateAutomaticStartAction(int(val))
	}

	if val, ok := vmData["AutomaticStartDelay"].(float64); ok {
		settings.AutomaticStartDelay = uint32(val)
	}

	if val, ok := vmData["AutomaticStopAction"].(float64); ok {
		settings.AutomaticStopAction = cerateAutomaticStopAction(int(val))
	}

	if val, ok := vmData["AutomaticCriticalErrorAction"].(float64); ok {
		settings.AutomaticCriticalErrorAction = cerateAutomaticCriticalErrorAction(int(val))
	}

	if val, ok := vmData["AutomaticCriticalErrorActionTimeout"].(float64); ok {
		settings.AutomaticCriticalErrorActionTimeout = uint32(val)
	}

	if val, ok := vmData["AutomaticCheckpointsEnabled"].(bool); ok {
		settings.AutomaticCheckpointsEnabled = val
	}

	if val, ok := vmData["CheckpointType"].(float64); ok {
		settings.CheckpointType = cerateCheckpointType(int(val))
	}

	if val, ok := vmData["SmartPagingFilePath"].(string); ok {
		settings.SmartPagingFilePath = val
	}

	if val, ok := vmData["SnapshotFileLocation"].(string); ok {
		settings.SnapshotFileLocation = val
	}

	if val, ok := vmData["LockOnDisconnect"].(float64); ok {
		settings.LockOnDisconnect = val !=1
	}

	if val, ok := vmData["Notes"].(string); ok {
		settings.Notes = val
	}

	if val, ok := vmData["GuestControlledCacheTypes"].(bool); ok {
		settings.GuestControlledCacheTypes = val
	}

	return settings, nil
}

func cerateAutomaticStartAction(val int) AutomaticStartActionType {
	switch val {
	case 2:
		return AutomaticStartActionNothing
	case 3:
		return AutomaticStartActionStartIfRunning
	case 4:
		return AutomaticStartActionStart
	default:
		return AutomaticStartActionNone
	}
}

func cerateAutomaticStopAction(val int) AutomaticStopActionType {
	switch val {
	case 2:
		return AutomaticStopActionTurnOff
	case 3:
		return AutomaticStopActionSave
	case 4:
		return AutomaticStopActionShutDown
	default:
		return AutomaticStopActionNone
	}
}

func cerateAutomaticCriticalErrorAction(val int) AutomaticCriticalErrorActionType {
	switch val {
	case 0:
		return AutomaticCriticalErrorActionNone
	case 1:
		return AutomaticCriticalErrorActionPause
	default:
		return AutomaticCriticalErrorActionNone
	}
}

func cerateCheckpointType(val int) CheckpointType {
	switch val {
	case 2:
		return CheckpointTypeDisabled
	case 3:
		return CheckpointTypeProduction
	case 4:
		return CheckpointTypeProductionOnly
	case 5:
		return CheckpointTypeStandard
	default:
		return CheckpointTypeNone
	}
}
