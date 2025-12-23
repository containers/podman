//go:build windows

package hypervctl

import (
	"strconv"

	"github.com/containers/libhvee/pkg/powershell"
)

func CheckHypervAvailable() error {
	if err := powershell.HypervAvailable(); err != nil {
		return err
	}

	if !powershell.IsHypervAdministrator() {
		return powershell.ErrNotAdministrator
	}
	return nil
}

// https://learn.microsoft.com/en-us/windows/win32/hyperv_v2/msvm-computersystem

type VirtualMachineManager struct {
}

func NewVirtualMachineManager() *VirtualMachineManager {
	return &VirtualMachineManager{}
}

func (vmm *VirtualMachineManager) GetAll() ([]*VirtualMachine, error) {
	psVMs, err := GetVMsFromPowerShell()
	if err != nil {
		return nil, err
	}
	vms := make([]*VirtualMachine, 0, len(psVMs))
	for _, psVM := range psVMs {
		vms = append(vms, psVM.ConvertToVirtualMachine(vmm))
	}
	return vms, nil
}

func (vmm *VirtualMachineManager) Exists(name string) (bool, error) {
	vms, err := vmm.GetAll()
	if err != nil {
		return false, err
	}
	for _, i := range vms {
		// TODO should case be honored or ignored?
		if i.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// GetMachine is a stub to lookup and get settings for a VM
func (vmm *VirtualMachineManager) GetMachine(name string) (*VirtualMachine, error) {
	return vmm.getMachine(name)
}

// GetMachineExists looks for a machine defined in hyperv and returns it if it exists
func (vmm *VirtualMachineManager) GetMachineExists(name string) (bool, *VirtualMachine, error) {
	vm, err := vmm.getMachine(name)
	if err != nil {
		return false, nil, nil
	}
	return true, vm, nil
}

// getMachine looks up a single VM by name
func (vmm *VirtualMachineManager) getMachine(name string) (*VirtualMachine, error) {
	psVM, err := GetVMFromPowerShell(name)
	if err != nil {
		return nil, err
	}
	return psVM.ConvertToVirtualMachine(vmm), nil
}

func (*VirtualMachineManager) CreateVhdxFile(path string, maxSize uint64) error {
	_, stderr, err := powershell.Execute("Hyper-V\\New-VHD", "-Path", path, "-SizeBytes", strconv.FormatUint(maxSize, 10))
	if err != nil {
		return NewPSError(stderr)
	}
	return nil
}

// GetSummaryInformation returns the live VM summary information for all virtual machines.
func (vmm *VirtualMachineManager) GetSummaryInformation() ([]PowerShellVM, error) {
	return vmm.getSummaryInformation("")
}

func (vmm *VirtualMachineManager) getSummaryInformation(vmName string) ([]PowerShellVM, error) {
	if vmName == "" {
		// Get all VMs
		psVMs, err := GetVMsFromPowerShell()
		if err != nil {
			return nil, err
		}

		return psVMs, nil
	} else {
		// Get specific VM
		psVM, err := GetVMFromPowerShell(vmName)
		if err != nil {
			return nil, err
		}

		return []PowerShellVM{*psVM}, nil
	}
}
