//go:build windows

package hypervctl

import (
	"fmt"
	"os"

	"github.com/containers/libhvee/pkg/powershell"
)

type SystemSettingsBuilder struct {
	systemSettings    *SystemSettings
	processorSettings *ProcessorSettings
	memorySettings    *MemorySettings
	err               error
}

func NewSystemSettingsBuilder() *SystemSettingsBuilder {
	return &SystemSettingsBuilder{}
}

func (builder *SystemSettingsBuilder) PrepareSystemSettings(name string, beforeAdd func(*SystemSettings)) *SystemSettingsBuilder {
	if builder.err != nil {
		return builder
	}

	if builder.systemSettings == nil {
		settings := DefaultSystemSettings()
		settings.Name = name
		builder.systemSettings = settings
	}

	if beforeAdd != nil {
		beforeAdd(builder.systemSettings)
	}

	return builder
}

func (builder *SystemSettingsBuilder) PrepareProcessorSettings(beforeAdd func(*ProcessorSettings)) *SystemSettingsBuilder {
	if builder.err != nil {
		return builder
	}

	if builder.processorSettings == nil {
		settings, err := fetchDefaultProcessorSettings()
		if err != nil {
			builder.err = err
			return builder
		}
		builder.processorSettings = settings
	}

	if beforeAdd != nil {
		beforeAdd(builder.processorSettings)
	}

	builder.processorSettings.VMName = builder.systemSettings.Name

	return builder
}

func (builder *SystemSettingsBuilder) PrepareMemorySettings(beforeAdd func(*MemorySettings)) *SystemSettingsBuilder {
	if builder.err != nil {
		return builder
	}

	if builder.memorySettings == nil {
		settings, err := fetchDefaultMemorySettings()
		if err != nil {
			builder.err = err
			return builder
		}
		builder.memorySettings = settings
	}

	if beforeAdd != nil {
		beforeAdd(builder.memorySettings)
	}

	builder.memorySettings.VMName = builder.systemSettings.Name

	return builder
}

func (builder *SystemSettingsBuilder) Build() (*SystemSettings, error) {

	var err error
	// New-VM add network adapter to the vm, so we need to remove it before setting the vm, as some users(podman) don't want to have a network adapter on the vm by default.
	_, stderr, err := powershell.Execute("Hyper-V\\New-VM", "-Name", builder.systemSettings.Name, "-Generation", "2", "|", "Get-VMNetworkAdapter", "|", "Remove-VMNetworkAdapter")

	if err != nil {
		return nil, NewPSError(stderr)
	}

	cliArgs := getCLI(builder.systemSettings)
	args := append([]string{"Hyper-V\\Set-VM"}, cliArgs...)
	_, stderr, err = powershell.Execute(args...)

	if err != nil {
		fmt.Fprintf(os.Stderr, "error setting vm: %s\n", err)
		return nil, NewPSError(stderr)
	}

	_, stderr, err = powershell.Execute("Hyper-V\\Set-VMFirmware", "-VMName", builder.systemSettings.Name, "-PreferredNetworkBootProtocol", string(builder.systemSettings.NetworkBootPreferredProtocol), "-EnableSecureBoot", "Off")

	if err != nil {
		fmt.Fprintf(os.Stderr, "error setting vm firmware: %s\n", err)
		return nil, NewPSError(stderr)
	}

	err = updateVMMemory(builder.systemSettings.Name, builder.memorySettings)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error setting vm memory: %s\n", err)
		return nil, err
	}

	err = updateVMProcessor(builder.systemSettings.Name, builder.processorSettings)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error setting vm processor: %s\n", err)
		return nil, err
	}

	return GetVMFromName(builder.systemSettings.Name)
}
