//go:build windows

package hypervctl

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/containers/libhvee/pkg/powershell"
)

const defaultSwitchName = "'Default Switch'"

// AddVMNetworkAdapterParams represents the arguments for the PowerShell cmdlet
// "Add-VMNetworkAdapter". This type can be used to add a new network adapter
// to a Hyper-V virtual machine.
//
// Pointers are used for optional fields to differentiate between a zero value
// and a field that was not explicitly provided.
type NetworkAdapterParams struct {
	// VMName specifies the name of the virtual machine to which the network adapter
	// is to be added. This is a mandatory parameter in one of the parameter sets.
	VMName string `json:"vmName,omitempty"`

	// Name assigns a name to the new virtual network adapter (default is "Network Adapter").
	Name string `json:"name,omitempty"`

	// SwitchName specifies the name of the virtual switch to connect the new adapter to.
	SwitchName string `json:"switchName,omitempty"`

	// ResourcePoolName specifies the friendly name of a resource pool.
	ResourcePoolName string `json:"resourcePoolName,omitempty"`

	// StaticMacAddress assigns a specific MAC address to the new adapter.
	StaticMacAddress string `json:"staticMacAddress,omitempty"`

	// DynamicMacAddress assigns a dynamically generated MAC address to the new adapter.
	// This is a switch parameter, so a pointer to a bool is used.
	DynamicMacAddress bool `json:"dynamicMacAddress,omitempty"`

	// IsLegacy specifies if the virtual network adapter is a legacy type.
	IsLegacy bool `json:"isLegacy,omitempty"`

	// ManagementOS specifies that the adapter should be added to the management OS.
	// This is a switch parameter.
	ManagementOS bool `json:"managementOS,omitempty"`

	// DeviceNaming adds a virtual network adapter to a virtual machine.
	// This is a switch parameter.
	DeviceNaming bool `json:"deviceNaming,omitempty"`

	// Passthru passes the object representing the new network adapter to the pipeline.
	Passthru bool `json:"passthru,omitempty"`

	// Other optional parameters for remote sessions or confirmation.
	ComputerName string `json:"computerName,omitempty"`
	Confirm      bool   `json:"confirm,omitempty"`
}

func (n *NetworkAdapterParams) DefineEthernetPortConnection(switchName string) (*NetworkAdapterParams, error) {
	n.SwitchName = switchName
	adapterStr := n.ToAddVMNetworkAdapterArgs()
	args := append([]string{"Hyper-V\\Add-VMNetworkAdapter"}, adapterStr...)
	_, stderr, err := powershell.Execute(args...)
	if err != nil {
		return nil, NewPSError(stderr)
	}

	adapters, err := GetVMNetworkAdapterAndParse(n.VMName)
	if err != nil {
		return nil, err
	}

	return adapters[len(adapters)-1], nil
}

// ToAddVMNetworkAdapterArgs converts NetworkAdapterParams to CLI arguments for Add-VMNetworkAdapter
func (n *NetworkAdapterParams) ToAddVMNetworkAdapterArgs() []string {
	var args []string

	// Required parameter - VMName
	if n.VMName != "" {
		args = append(args, "-VMName", fmt.Sprintf("'%s'", n.VMName))
	}

	// Optional parameters
	if n.Name != "" {
		args = append(args, "-Name", fmt.Sprintf("'%s'", n.Name))
	}

	if n.SwitchName != "" {
		args = append(args, "-SwitchName", fmt.Sprintf("'%s'", n.SwitchName))
	} else {
		args = append(args, "-SwitchName", defaultSwitchName)
	}

	if n.ResourcePoolName != "" {
		args = append(args, "-ResourcePoolName", fmt.Sprintf("'%s'", n.ResourcePoolName))
	}

	if n.StaticMacAddress != "" {
		args = append(args, "-StaticMacAddress", fmt.Sprintf("'%s'", n.StaticMacAddress))
	}

	// Switch parameters (boolean flags)
	if n.DynamicMacAddress {
		args = append(args, "-DynamicMacAddress")
	}

	if n.IsLegacy {
		args = append(args, "-IsLegacy", "$True")
	}

	if n.ManagementOS {
		args = append(args, "-ManagementOS")
	}

	if n.DeviceNaming {
		args = append(args, "-DeviceNaming", "On")
	}

	if n.Passthru {
		args = append(args, "-Passthru")
	}

	if n.ComputerName != "" {
		args = append(args, "-ComputerName", fmt.Sprintf("'%s'", n.ComputerName))
	}

	if n.Confirm {
		args = append(args, "-Confirm")
	}

	return args
}

// GetVMNetworkAdapterAndParse calls Get-VMNetworkAdapter CLI, parses output and creates NetworkAdapterParams
func GetVMNetworkAdapterAndParse(vmName string) ([]*NetworkAdapterParams, error) {
	// Build the Get-VMNetworkAdapter command
	var args []string
	args = append(args, "Get-VMNetworkAdapter")

	if vmName != "" {
		args = append(args, "-VMName", fmt.Sprintf("'%s'", vmName))
	}

	// Convert to JSON for easier parsing
	args = append(args, "|", "ConvertTo-Json", "-Depth", "3")

	cmd := strings.Join(args, " ")

	// Execute the PowerShell command
	stdout, stderr, err := powershell.Execute(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to execute Get-VMNetworkAdapter: %w", NewPSError(stderr))
	}

	// Parse JSON output
	stdout = strings.TrimSpace(stdout)
	if stdout == "" || stdout == "null" {
		return []*NetworkAdapterParams{}, nil
	}

	// Handle both single object and array responses
	var rawData interface{}
	if err := json.Unmarshal([]byte(stdout), &rawData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON output: %w", err)
	}

	var adapters []*NetworkAdapterParams

	// Check if it's an array or single object
	switch data := rawData.(type) {
	case []interface{}:
		// Multiple adapters
		for _, item := range data {
			adapter, err := parseNetworkAdapterFromMap(item)
			if err != nil {
				return nil, fmt.Errorf("failed to parse network adapter: %w", err)
			}
			adapters = append(adapters, adapter)
		}
	case map[string]interface{}:
		// Single adapter
		adapter, err := parseNetworkAdapterFromMap(data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse network adapter: %w", err)
		}
		adapters = append(adapters, adapter)
	default:
		return nil, fmt.Errorf("unexpected JSON structure")
	}

	return adapters, nil
}

// parseNetworkAdapterFromMap converts a map[string]interface{} to NetworkAdapterParams
func parseNetworkAdapterFromMap(data interface{}) (*NetworkAdapterParams, error) {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected map[string]interface{}, got %T", data)
	}

	adapter := &NetworkAdapterParams{}

	// Map PowerShell property names to NetworkAdapterParams fields
	if vmName, exists := dataMap["VMName"]; exists {
		if str, ok := vmName.(string); ok {
			adapter.VMName = str
		}
	}

	if name, exists := dataMap["Name"]; exists {
		if str, ok := name.(string); ok {
			adapter.Name = str
		}
	}

	if switchName, exists := dataMap["SwitchName"]; exists {
		if str, ok := switchName.(string); ok {
			adapter.SwitchName = str
		}
	}

	if poolName, exists := dataMap["PoolName"]; exists {
		if str, ok := poolName.(string); ok {
			adapter.ResourcePoolName = str
		}
	}

	if macAddress, exists := dataMap["MacAddress"]; exists {
		if str, ok := macAddress.(string); ok {
			adapter.StaticMacAddress = str
		}
	}

	// Check if MAC address is dynamic
	if dynamicMac, exists := dataMap["DynamicMacAddressEnabled"]; exists {
		if b, ok := dynamicMac.(bool); ok {
			adapter.DynamicMacAddress = b
		}
	}

	if isLegacy, exists := dataMap["IsLegacy"]; exists {
		if b, ok := isLegacy.(bool); ok {
			adapter.IsLegacy = b
		}
	}

	if managementOS, exists := dataMap["IsManagementOs"]; exists {
		if b, ok := managementOS.(bool); ok {
			adapter.ManagementOS = b
		}
	}

	if deviceNaming, exists := dataMap["DeviceNaming"]; exists {
		if f, ok := deviceNaming.(float64); ok {
			adapter.DeviceNaming = (f == 0)
		}
	}

	if computerName, exists := dataMap["ComputerName"]; exists {
		if str, ok := computerName.(string); ok {
			adapter.ComputerName = str
		}
	}

	return adapter, nil
}
