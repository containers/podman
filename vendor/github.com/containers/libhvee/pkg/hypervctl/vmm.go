//go:build windows
// +build windows

package hypervctl

import (
	"fmt"

	"github.com/containers/libhvee/pkg/wmiext"
)

const (
	HyperVNamespace                = "root\\virtualization\\v2"
	VirtualSystemManagementService = "Msvm_VirtualSystemManagementService"
)

// https://learn.microsoft.com/en-us/windows/win32/hyperv_v2/msvm-computersystem

type VirtualMachineManager struct {
}

func NewVirtualMachineManager() *VirtualMachineManager {
	return &VirtualMachineManager{}
}

func (vmm *VirtualMachineManager) GetAll() ([]*VirtualMachine, error) {
	const wql = "Select * From Msvm_ComputerSystem Where Description = 'Microsoft Virtual Machine'"

	var service *wmiext.Service
	var err error
	if service, err = wmiext.NewLocalService(HyperVNamespace); err != nil {
		return []*VirtualMachine{}, err
	}
	defer service.Close()

	var enum *wmiext.Enum
	if enum, err = service.ExecQuery(wql); err != nil {
		return nil, err
	}
	defer enum.Close()

	var vms []*VirtualMachine
	for {
		vm := &VirtualMachine{vmm: vmm}
		done, err := wmiext.NextObject(enum, vm)
		if err != nil {
			return vms, err
		}
		if done {
			break
		}
		vms = append(vms, vm)
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

func (*VirtualMachineManager) GetMachine(name string) (*VirtualMachine, error) {
	const wql = "Select * From Msvm_ComputerSystem Where Description = 'Microsoft Virtual Machine' And ElementName='%s'"

	vm := &VirtualMachine{}
	var service *wmiext.Service
	var err error

	if service, err = wmiext.NewLocalService(HyperVNamespace); err != nil {
		return vm, err
	}
	defer service.Close()

	var enum *wmiext.Enum
	if enum, err = service.ExecQuery(fmt.Sprintf(wql, name)); err != nil {
		return nil, err
	}
	defer enum.Close()

	done, err := wmiext.NextObject(enum, vm)
	if err != nil {
		return vm, err
	}

	if done {
		return vm, fmt.Errorf("could not find virtual machine %q", name)
	}

	return vm, nil
}

func (*VirtualMachineManager) CreateVhdxFile(path string, maxSize uint64) error {
	var service *wmiext.Service
	var err error
	if service, err = wmiext.NewLocalService(HyperVNamespace); err != nil {
		return err
	}
	defer service.Close()

	settings := &VirtualHardDiskSettings{}
	settings.Format = 3
	settings.MaxInternalSize = maxSize
	settings.Type = 3
	settings.Path = path

	instance, err := service.CreateInstance("Msvm_VirtualHardDiskSettingData", settings)
	if err != nil {
		return err
	}
	defer instance.Close()
	settingsStr := instance.GetCimText()

	imms, err := service.GetSingletonInstance("Msvm_ImageManagementService")
	if err != nil {
		return err
	}
	defer imms.Close()

	var job *wmiext.Instance
	var ret int32
	err = imms.BeginInvoke("CreateVirtualHardDisk").
		In("VirtualDiskSettingData", settingsStr).
		Execute().
		Out("Job", &job).
		Out("ReturnValue", &ret).
		End()

	if err != nil {
		return fmt.Errorf("failed to create vhdx: %w", err)
	}

	return waitVMResult(ret, service, job, "failed to create vhdx", nil)
}

// GetSummaryInformation returns the live VM summary information for all virtual machines.
// The requestedFields parameter controls which fields of summary information are populated.
// SummaryRequestCommon and SummaryRequestNearAll provide predefined combinations for this
// parameter.
func (vmm *VirtualMachineManager) GetSummaryInformation(requestedFields SummaryRequestSet) ([]SummaryInformation, error) {
	return vmm.getSummaryInformation("", requestedFields)
}

func (vmm *VirtualMachineManager) getSummaryInformation(settingsPath string, requestedFields SummaryRequestSet) ([]SummaryInformation, error) {
	var service *wmiext.Service
	var err error
	if service, err = wmiext.NewLocalService(HyperVNamespace); err != nil {
		return nil, err
	}
	defer service.Close()

	vsms, err := service.GetSingletonInstance(VirtualSystemManagementService)
	if err != nil {
		return nil, err
	}
	defer vsms.Close()

	var summary []SummaryInformation

	inv := vsms.BeginInvoke("GetSummaryInformation").
		In("RequestedInformation", []uint(requestedFields))

	if len(settingsPath) > 0 {
		inv.In("SettingData", []string{settingsPath})
	}

	err = inv.Execute().
		Out("SummaryInformation", &summary).
		End()

	if err != nil {
		return nil, err
	}

	return summary, nil
}
