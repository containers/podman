//go:build windows

package hypervctl

import (
	"encoding/json"
	"fmt"

	"github.com/containers/libhvee/pkg/powershell"
)

// ProcessorSettings represents the arguments for the PowerShell cmdlet
// "Set-VMProcessor". This type can be used to configure virtual processor
// settings for a Hyper-V virtual machine.
//
// Pointers are used for optional fields to differentiate between a zero value
// and a field that was not explicitly provided.
type ProcessorSettings struct {
	// VMName specifies the name of the virtual machine. This parameter
	// is typically used to identify the target VM.
	VMName string `json:"vmName,omitempty"`

	// Count specifies the number of virtual processors for the virtual machine.
	Count int64 `json:"count,omitempty"`

	// Maximum specifies the maximum percentage of processor resources
	// available to the virtual machine (0-100).
	Maximum int64 `json:"maximum,omitempty"`

	// Reserve specifies the percentage of processor resources to be reserved
	// for the virtual machine (0-100).
	Reserve int64 `json:"reserve,omitempty"`

	// RelativeWeight specifies the priority for allocating physical CPU
	// time to this VM relative to others (1-10000).
	RelativeWeight int `json:"relativeWeight,omitempty"`

	// CompatibilityForMigrationEnabled enables or disables compatibility mode for
	// live migration between hosts with different processor versions.
	CompatibilityForMigrationEnabled bool `json:"compatibilityForMigrationEnabled,omitempty"`

	// CompatibilityForOlderOperatingSystemsEnabled enables or disables
	// compatibility for older guest operating systems.
	CompatibilityForOlderOperatingSystemsEnabled bool `json:"compatibilityForOlderOperatingSystemsEnabled,omitempty"`

	// EnableHostResourceProtection specifies whether to enable host resource
	// protection to prevent excessive resource consumption.
	EnableHostResourceProtection bool `json:"enableHostResourceProtection,omitempty"`

	// ExposeVirtualizationExtensions enables nested virtualization by
	// exposing virtualization extensions to the guest VM.
	ExposeVirtualizationExtensions bool `json:"exposeVirtualizationExtensions,omitempty"`

	// HwThreadCountPerCore specifies the number of virtual SMT threads
	// exposed to the virtual machine.
	HwThreadCountPerCore int64 `json:"hwThreadCountPerCore,omitempty"`

	// MaximumCountPerNumaNode specifies the maximum number of processors
	// per NUMA node.
	MaximumCountPerNumaNode int `json:"maximumCountPerNumaNode,omitempty"`

	// MaximumCountPerNumaSocket specifies the maximum number of sockets
	// per NUMA node.
	MaximumCountPerNumaSocket int `json:"maximumCountPerNumaSocket,omitempty"`

	// ResourcePoolName specifies the name of the processor resource pool.
	ResourcePoolName string `json:"resourcePoolName,omitempty"`

	// ComputerName specifies one or more Hyper-V hosts on which to
	// configure the VM processor.
	ComputerName string `json:"computerName,omitempty"`
}

func fetchDefaultProcessorSettings() (*ProcessorSettings, error) {
	settings := &ProcessorSettings{}
	return settings, nil
}

func updateVMProcessor(vmName string, settings *ProcessorSettings) error {
	processorStr := getProcessorCLI(settings)
	args := append([]string{"Hyper-V\\Set-VMProcessor"}, processorStr...)
	_, stderr, err := powershell.Execute(args...)
	if err != nil {
		return NewPSError(stderr)
	}
	return nil
}	

// getProcessorCLI generates PowerShell Set-VMProcessor command parameters from ProcessorSettings
func getProcessorCLI(settings *ProcessorSettings) []string {
	if settings.VMName == "" {
		return []string{}
	}

	params := []string{}

	// VM Name is required
	params = append(params, "-VMName", fmt.Sprintf("'%s'", settings.VMName))

	// Boolean parameters
	if settings.CompatibilityForMigrationEnabled {
		params = append(params, "-CompatibilityForMigrationEnabled", "$True")
	}
	if settings.CompatibilityForOlderOperatingSystemsEnabled {
		params = append(params, "-CompatibilityForOlderOperatingSystemsEnabled", "$True")
	}
	if settings.EnableHostResourceProtection {
		params = append(params, "-EnableHostResourceProtection", "$True")
	}
	if settings.ExposeVirtualizationExtensions {
		params = append(params, "-ExposeVirtualizationExtensions", "$True")
	}

	// Numeric parameters (only add if > 0)
	if settings.Count > 0 {
		params = append(params, "-Count", fmt.Sprintf("%d", settings.Count))
	}
	if settings.Maximum > 0 {
		params = append(params, "-Maximum", fmt.Sprintf("%d", settings.Maximum))
	}
	if settings.Reserve > 0 {
		params = append(params, "-Reserve", fmt.Sprintf("%d", settings.Reserve))
	}
	if settings.RelativeWeight > 0 {
		params = append(params, "-RelativeWeight", fmt.Sprintf("%d", settings.RelativeWeight))
	}
	if settings.HwThreadCountPerCore > 0 {
		params = append(params, "-HwThreadCountPerCore", fmt.Sprintf("%d", settings.HwThreadCountPerCore))
	}
	if settings.MaximumCountPerNumaNode > 0 {
		params = append(params, "-MaximumCountPerNumaNode", fmt.Sprintf("%d", settings.MaximumCountPerNumaNode))
	}
	if settings.MaximumCountPerNumaSocket > 0 {
		params = append(params, "-MaximumCountPerNumaSocket", fmt.Sprintf("%d", settings.MaximumCountPerNumaSocket))
	}

	// String parameters
	if settings.ResourcePoolName != "" {
		params = append(params, "-ResourcePoolName", fmt.Sprintf("'%s'", settings.ResourcePoolName))
	}
	if settings.ComputerName != "" {
		params = append(params, "-ComputerName", fmt.Sprintf("'%s'", settings.ComputerName))
	}

	return params
}

// GetVMProcessorFromName executes Get-VMProcessor PowerShell command and parses JSON output into ProcessorSettings
func GetVMProcessorFromName(vmName string) (*ProcessorSettings, error) {
	if vmName == "" {
		return nil, fmt.Errorf("VM name cannot be empty")
	}

	// Execute PowerShell command to get VM processor info as JSON
	cmd := fmt.Sprintf("Get-VMProcessor -VMName '%s' | ConvertTo-Json", vmName)
	stdout, stderr, err := powershell.Execute(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to execute PowerShell command: %w, stderr: %s", err, stderr)
	}

	// Parse JSON response
	var processorData map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &processorData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON output: %w", err)
	}

	// Create ProcessorSettings instance and map fields
	settings := &ProcessorSettings{
		VMName: vmName,
	}

	// Map PowerShell VM processor properties to ProcessorSettings fields
	if val, ok := processorData["Count"].(float64); ok {
		settings.Count = int64(val)
	}

	if val, ok := processorData["Maximum"].(float64); ok {
		settings.Maximum = int64(val)
	}

	if val, ok := processorData["Reserve"].(float64); ok {
		settings.Reserve = int64(val)
	}

	if val, ok := processorData["RelativeWeight"].(float64); ok {
		settings.RelativeWeight = int(val)
	}

	if val, ok := processorData["CompatibilityForMigrationEnabled"].(bool); ok {
		settings.CompatibilityForMigrationEnabled = val
	}

	if val, ok := processorData["CompatibilityForOlderOperatingSystemsEnabled"].(bool); ok {
		settings.CompatibilityForOlderOperatingSystemsEnabled = val
	}

	if val, ok := processorData["EnableHostResourceProtection"].(bool); ok {
		settings.EnableHostResourceProtection = val
	}

	if val, ok := processorData["ExposeVirtualizationExtensions"].(bool); ok {
		settings.ExposeVirtualizationExtensions = val
	}

	if val, ok := processorData["HwThreadCountPerCore"].(float64); ok {
		settings.HwThreadCountPerCore = int64(val)
	}

	if val, ok := processorData["MaximumCountPerNumaNode"].(float64); ok {
		settings.MaximumCountPerNumaNode = int(val)
	}

	if val, ok := processorData["MaximumCountPerNumaSocket"].(float64); ok {
		settings.MaximumCountPerNumaSocket = int(val)
	}

	if val, ok := processorData["ResourcePoolName"].(string); ok {
		settings.ResourcePoolName = val
	}

	return settings, nil
}
