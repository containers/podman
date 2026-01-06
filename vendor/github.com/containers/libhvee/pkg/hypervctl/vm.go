//go:build windows

package hypervctl

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/containers/libhvee/pkg/kvp/ginsu"
	"github.com/containers/libhvee/pkg/powershell"
)

// delete this when close to being done
var (
	ErrNotImplemented = errors.New("function not implemented")
)

type VirtualMachine struct {
	PowerShellVM
	vmm *VirtualMachineManager
}

func (vm *VirtualMachine) GetName() string {
	return vm.Name
}

func (vm *VirtualMachine) SplitAndAddIgnition(keyPrefix string, ignRdr *bytes.Reader) error {
	parts, err := ginsu.Dice(ignRdr)
	if err != nil {
		return err
	}
	for idx, val := range parts {
		key := fmt.Sprintf("%s%d", keyPrefix, idx)
		if err := vm.AddKeyValuePair(key, val); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VirtualMachine) AddKeyValuePair(key string, value string) error {
	return vm.kvpOperation("AddKvpItems", key, value)
}

func (vm *VirtualMachine) ModifyKeyValuePair(key string, value string) error {
	return vm.kvpOperation("ModifyKvpItems", key, value)
}

func (vm *VirtualMachine) PutKeyValuePair(key string, value string) error {
	err := vm.AddKeyValuePair(key, value)

	if err != nil && !strings.Contains(err.Error(), "The parameter is incorrect. (0x80070057)") {
		return err
	}

	return vm.ModifyKeyValuePair(key, value)
}

func (vm *VirtualMachine) RemoveKeyValuePair(key string) error {
	return vm.kvpOperation("RemoveKvpItems", key, "")
}

func (vm *VirtualMachine) RemoveKeyValuePairNoWait(key string) error {
	return vm.kvpOperation("RemoveKvpItems", key, "")
}

func (vm *VirtualMachine) GetKeyValuePairs() (map[string]string, error) {
	script := getAllKvpItemsScript(vm.Name)
	_, stderr, err := powershell.ExecuteAsScript(script)
	if err != nil {
		return nil, NewPSError(stderr)
	}
	return parseKvpMapXml(stderr)
}

func (vm *VirtualMachine) kvpOperation(op string, key string, value string) error {
	script := getKvpScript(vm.Name, op, key, value)
	_, stderr, err := powershell.ExecuteAsScript(script)
	if err != nil {
		return NewPSError(stderr)
	}
	return nil

}

func getKvpScript(vmName string, op string, key string, value string) []string {
	return []string{
		`Import-Module CimCmdlets`,
		`# Connect to the VM`,
		`$vmMgmt = Get-WmiObject -Namespace "root\virtualization\v2" -Class Msvm_VirtualSystemManagementService`,
		`$vm = Get-WmiObject -Namespace "root\virtualization\v2" -Class Msvm_ComputerSystem -Filter "ElementName='` + vmName + `'"`,
		`# Create KVP data item`,
		`$kvpClass = [WMIClass]("\\$($vmMgmt.__SERVER)\$($vmMgmt.__NAMESPACE):Msvm_KvpExchangeDataItem")`,
		`$kvpDataItem = $kvpClass.CreateInstance()`,
		`$kvpDataItem.Name = "` + key + `"`,
		`$kvpDataItem.Data = '` + strings.ReplaceAll(value, "'", "''") + `'`,
		`$kvpDataItem.Source = 0`,

		`# Add (or modify) the KVP`,
		`$result = $vmMgmt.` + op + `($vm, $kvpDataItem.PSBase.GetText(1))`,

		`# Check if a job was started`,
		`if ($result.ReturnValue -eq 4096) {`,
		`# Extract the InstanceID from the Job path`,
		`$instanceId = [regex]::Match($result.Job, 'InstanceID="(.+?)"').Groups[1].Value`,

		`# Construct the query correctly by using single quotes to escape the quotes in InstanceID`,
		`$query = "SELECT * FROM Msvm_ConcreteJob WHERE InstanceID = '$instanceId'"`,

		`# Get the job object using the corrected query`,
		`$job = Get-WmiObject -Namespace "root\virtualization\v2" -Query $query`,

		`# Wait for the job to complete`,
		`do {`,
		`Write-Host "Waiting for the KVP item to be ` + op + `..."`,
		`Start-Sleep -Seconds 1`,

		`# Refresh the job object to get its latest state`,
		`$job = Get-WmiObject -Namespace "root\virtualization\v2" -Query $query`,
		`} until ($job.JobState -ne 4)`,

		`# Check the final status of the job`,
		`if ($job.JobState -eq 7) {`,
		`Write-Host "Successfully ` + op + ` the KVP item."`,
		`} else {`,
		`Write-Error "Failed to ` + op + ` the KVP item. JobState: $($job.JobState)"`,
		`Write-Error "Error Description: $($job.ErrorDescription)"`,
		`}`,
		`} else {`,
		`Write-Error "The ` + op + ` method did not start an asynchronous job. Return value: $($result.ReturnValue)"`,
		`}`,
	}

}

func getAllKvpItemsScript(vmName string) []string {
	return []string{
		`$vm = Get-WmiObject -Namespace root\virtualization\v2 -Class  Msvm_ComputerSystem -Filter { ElementName='` + vmName + `' }`,
		`write-output ($vm.GetRelated("Msvm_KvpExchangeComponent")[0]).GetRelated("Msvm_KvpExchangeComponentSettingData").HostExchangeItems`,
	}
}

func (vm *VirtualMachine) StopWithForce() error {
	return vm.stop(true)
}

func (vm *VirtualMachine) Stop() error {
	return vm.stop(false)
}

func (vm *VirtualMachine) stop(force bool) error {
	if !Enabled.equal(uint16(vm.State)) {
		return ErrMachineNotRunning
	}

	args := []string{"Hyper-V\\Stop-VM", "-Name", vm.Name, "-Force"}
	if force {
		args = append(args, "-TurnOff")
	}

	_, stderr, err := powershell.Execute(args...)
	if err != nil {
		return NewPSError(stderr)
	}
	return nil
}

func (vm *VirtualMachine) Start() error {

	if s := vm.State; !Disabled.equal(uint16(s)) {
		if Enabled.equal(uint16(s)) {
			return ErrMachineAlreadyRunning
		} else if Starting.equal(uint16(s)) {
			return ErrMachineAlreadyRunning
		}
		return errors.New("machine not in a state to start")
	}

	_, stderr, err := powershell.Execute("Hyper-V\\Start-VM", "-Name", vm.Name)
	if err != nil {
		return NewPSError(stderr)
	}
	return nil
}

func (vm *VirtualMachine) GetConfig(diskPath string) (*HyperVConfig, error) {
	var (
		diskSize uint64
	)
	summary, err := vm.GetSummaryInformation()
	if err != nil {
		return nil, err
	}

	// Grabbing actual disk size
	diskPathInfo, err := os.Stat(diskPath)
	if err != nil {
		return nil, err
	}
	diskSize = uint64(diskPathInfo.Size())
	mem, err := vm.getMemorySettings()
	if err != nil {
		return nil, err
	}

	config := HyperVConfig{
		Hardware: HardwareConfig{
			// TODO we could implement a getProcessorSettings like we did for memory
			CPUs:     uint16(summary.ProcessorCount),
			DiskPath: diskPath,
			DiskSize: diskSize,
			Memory:   mem.StartupBytes,
		},
		Status: Statuses{
			Created:  summary.ConvertToCreationTime(),
			LastUp:   time.Time{},
			Running:  Enabled == summary.State,
			Starting: vm.IsStarting(),
			State:    EnabledState(summary.State),
		},
	}
	return &config, nil
}

// GetSummaryInformation returns the live VM summary information for this virtual machine.
func (vm *VirtualMachine) GetSummaryInformation() (*PowerShellVM, error) {

	result, err := vm.vmm.getSummaryInformation(vm.Name)
	if err != nil {
		return nil, err
	}

	if len(result) < 1 {
		return nil, errors.New("summary information search returned an empty result set")
	}

	return &result[0], nil
}

// NewVirtualMachine creates a new vm in hyperv
// decided to not return a *VirtualMachine here because of how Podman is
// likely to use this.  this could be easily added if desirable
func (vmm *VirtualMachineManager) NewVirtualMachine(name string, config *HardwareConfig) error {
	exists, err := vmm.Exists(name)
	if err != nil {
		return err
	}
	if exists {
		return ErrMachineAlreadyExists
	}

	// TODO I gotta believe there are naming restrictions for vms in hyperv?
	// TODO If something fails during creation, do we rip things down or follow precedent from other machines?  user deletes things

	systemSettings, err := NewSystemSettingsBuilder().
		PrepareSystemSettings(name, nil).
		PrepareMemorySettings(func(ms *MemorySettings) {
			//ms.DynamicMemoryEnabled = false
			//ms.VirtualQuantity = 8192 // Startup memory
			//ms.Reservation = config.Memory // min

			// The API seems to require both of these even
			// when not using dynamic memory
			ms.MaximumBytes = config.Memory * 1024 * 1024
			ms.StartupBytes = config.Memory * 1024 * 1024
		}).
		PrepareProcessorSettings(func(ps *ProcessorSettings) {
			ps.Count = int64(config.CPUs) // 4 cores
		}).
		Build()
	if err != nil {
		return err
	}

	builder := NewDriveSettingsBuilder(systemSettings).
		AddScsiController().
		AddSyntheticDiskDrive(0).
		DefineVirtualHardDisk(config.DiskPath, func(vhdss *HardDiskDriveSettings) {
			// set extra params like
			// vhdss.IOPSLimit = 5000
		}).
		Finish(). // disk
		Finish()  // drive

	if config.DVDDiskPath != "" {
		// Add a DVD drive if the DVDDiskPath is set
		// This is useful for cloud-init or other bootable media
		builder = builder.
			AddSyntheticDvdDrive(1).
			DefineVirtualDvdDisk(config.DVDDiskPath).
			Finish(). // disk
			Finish()  // drive
	}

	if err := builder.
		Finish(). // controller
		Complete(); err != nil {
		return err
	}

	// Add default network connection
	if config.Network {
		if err := NewNetworkSettingsBuilder(systemSettings).
			AddSyntheticEthernetPort(nil).
			AddEthernetPortAllocation(""). // "" = connect to default switch
			Finish().                      // allocation
			Finish().                      // port
			Complete(); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VirtualMachine) getMemorySettings() (*MemorySettings, error) {
	memory, err := GetVMMemoryFromName(vm.Name)
	if err != nil {
		return nil, err
	}
	return memory, nil
}

// Update processor and/or mem
func (vm *VirtualMachine) UpdateProcessorMemSettings(updateProcessor func(*ProcessorSettings), updateMemory func(*MemorySettings)) error {

	if updateProcessor != nil {
		proc, err := GetVMProcessorFromName(vm.Name)

		if err != nil {
			return err
		}

		updateProcessor(proc)

		err = updateVMProcessor(vm.Name, proc)
		if err != nil {
			return err
		}
	}

	if updateMemory != nil {
		mem, err := GetVMMemoryFromName(vm.Name)
		if err != nil {
			return err
		}

		updateMemory(mem)

		err = updateVMMemory(vm.Name, mem)
		if err != nil {
			return err
		}
	}

	return nil
}

func (vm *VirtualMachine) remove() (int32, error) {

	refreshVM, err := vm.vmm.GetMachine(vm.Name)
	if err != nil {
		return 0, err
	}

	// Check for disabled/stopped state
	if !Disabled.equal(uint16(refreshVM.GetState())) {
		return -1, ErrMachineStateInvalid
	}
	_, stderr, err := powershell.Execute("Hyper-V\\Remove-VM", "-Name", vm.Name, "-Force")
	if err != nil {
		return -1, NewPSError(stderr)
	}
	return 0, nil
}

func (vm *VirtualMachine) Remove(diskPath string) error {
	if _, err := vm.remove(); err != nil {
		return err
	}

	// Remove disk only if we were given one
	if len(diskPath) > 0 {
		if err := os.Remove(diskPath); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VirtualMachine) GetState() EnabledState {
	return EnabledState(vm.State)
}

func (vm *VirtualMachine) IsStarting() bool {
	return Starting.equal(uint16(vm.State))
}
