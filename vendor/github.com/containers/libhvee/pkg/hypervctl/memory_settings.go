//go:build windows

package hypervctl

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/containers/libhvee/pkg/powershell"
)

// SetVMMemoryParams represents the arguments for the PowerShell cmdlet
// "Set-VMMemory". This type can be used to configure memory settings
// for a Hyper-V virtual machine.
//
// Pointers are used for optional fields to differentiate between a zero value
// (e.g., an empty string or 0) and a field that was not provided.
type MemorySettings struct {
	// VMName specifies the name of the virtual machine to configure.
	// This is typically a required parameter in one of the parameter sets.
	VMName string `json:"vmName,omitempty"`

	// DynamicMemoryEnabled is a boolean parameter to enable or disable
	// dynamic memory for the virtual machine.
	DynamicMemoryEnabled bool `json:"dynamicMemoryEnabled,omitempty"`

	// MinimumBytes specifies the minimum amount of memory, in bytes,
	// that a virtual machine can be configured with when Dynamic Memory is enabled.
	MinimumBytes uint64 `json:"minimumBytes,omitempty"`

	// StartupBytes specifies the amount of memory, in bytes, that a
	// virtual machine will be allocated on startup.
	StartupBytes uint64 `json:"startupBytes,omitempty"`

	// MaximumBytes specifies the maximum amount of memory, in bytes,
	// that a virtual machine can be configured with when Dynamic Memory is enabled.
	MaximumBytes uint64 `json:"maximumBytes,omitempty"`

	// Buffer specifies the percentage of memory to be reserved as a buffer
	// within the virtual machine (allowed values 5 to 2000).
	Buffer int `json:"buffer,omitempty"`

	// Priority sets the priority for memory availability to this VM
	// relative to others (allowed values 0 to 100).
	Priority int `json:"priority,omitempty"`

	// ResourcePoolName specifies the name of the memory resource pool
	// for the virtual machine.
	ResourcePoolName string `json:"resourcePoolName,omitempty"`

	// ComputerName specifies one or more Hyper-V hosts on which to
	// configure the VM memory.
	ComputerName string `json:"computerName,omitempty"`
}

func fetchDefaultMemorySettings() (*MemorySettings, error) {
	settings := &MemorySettings{}
	return settings, nil
}

func updateVMMemory(vmName string, settings *MemorySettings) error {
	memoryStr := getMemoryCLI(vmName, settings)
	args := append([]string{"Hyper-V\\Set-VMMemory"}, memoryStr...)
	_, stderr, err := powershell.Execute(args...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error setting vm memory: %s\n", stderr)
		return NewPSError(stderr)
	}
	return nil
}

// getMemoryCLI generates PowerShell Set-VMMemory command parameters from MemorySettings
func getMemoryCLI(vmName string, settings *MemorySettings) []string {
	if vmName == "" {
		fmt.Fprintf(os.Stderr, "settings.VMName is empty\n")
		return []string{}
	}

	params := []string{}

	// VM Name is required
	params = append(params, "-VMName", fmt.Sprintf("'%s'", vmName))

	// Boolean parameters
	if settings.DynamicMemoryEnabled {
		params = append(params, "-DynamicMemoryEnabled", "$True")
	} else {
		params = append(params, "-DynamicMemoryEnabled", "$False")
	}

	// Numeric parameters (only add if > 0)

	if settings.StartupBytes > 0 {
		params = append(params, "-StartupBytes", fmt.Sprintf("%d", settings.StartupBytes))
	}

	// If Dynamic Memory is enabled, we can set the minimum, maximum, and buffer
	if settings.DynamicMemoryEnabled {
		if settings.MinimumBytes > 0 {
			params = append(params, "-MinimumBytes", fmt.Sprintf("%d", settings.MinimumBytes))
		}

		if settings.MaximumBytes > 0 {
			params = append(params, "-MaximumBytes", fmt.Sprintf("%d", settings.MaximumBytes))
		}
		if settings.Buffer > 0 {
			params = append(params, "-Buffer", fmt.Sprintf("%d", settings.Buffer))
		}
	}
	if settings.Priority > 0 {
		params = append(params, "-Priority", fmt.Sprintf("%d", settings.Priority))
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

// GetVMMemoryFromName executes Get-VMMemory PowerShell command and parses JSON output into MemorySettings
func GetVMMemoryFromName(vmName string) (*MemorySettings, error) {
	if vmName == "" {
		return nil, fmt.Errorf("VM name cannot be empty")
	}

	// Execute PowerShell command to get VM memory info as JSON
	cmd := fmt.Sprintf("Get-VMMemory -VMName '%s' | ConvertTo-Json", vmName)
	stdout, stderr, err := powershell.Execute(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to execute PowerShell command: %w, stderr: %s", err, stderr)
	}

	// Parse JSON response
	var memoryData map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &memoryData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON output: %w", err)
	}

	// Create MemorySettings instance and map fields
	settings := &MemorySettings{
		VMName: vmName,
	}

	// Map PowerShell VM memory properties to MemorySettings fields
	if val, ok := memoryData["DynamicMemoryEnabled"].(bool); ok {
		settings.DynamicMemoryEnabled = val
	}

	if val, ok := memoryData["Minimum"].(float64); ok {
		settings.MinimumBytes = uint64(val)
	}

	if val, ok := memoryData["Startup"].(float64); ok {
		settings.StartupBytes = uint64(val)
	}

	if val, ok := memoryData["Maximum"].(float64); ok {
		settings.MaximumBytes = uint64(val)
	}

	if val, ok := memoryData["Buffer"].(float64); ok {
		settings.Buffer = int(val)
	}

	if val, ok := memoryData["Priority"].(float64); ok {
		settings.Priority = int(val)
	}

	if val, ok := memoryData["ResourcePoolName"].(string); ok {
		settings.ResourcePoolName = val
	}

	return settings, nil
}
