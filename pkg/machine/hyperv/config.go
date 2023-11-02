//go:build windows
// +build windows

package hyperv

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/containers/libhvee/pkg/hypervctl"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
)

type HyperVVirtualization struct {
	machine.Virtualization
}

func VirtualizationProvider() machine.VirtProvider {
	return &HyperVVirtualization{
		machine.NewVirtualization(machine.HyperV, machine.Zip, machine.Vhdx, vmtype),
	}
}

func (v HyperVVirtualization) CheckExclusiveActiveVM() (bool, string, error) {
	vmm := hypervctl.NewVirtualMachineManager()
	// Use of GetAll is OK here because we do not want to use the same name
	// as something already *actually* configured in hyperv
	vms, err := vmm.GetAll()
	if err != nil {
		return false, "", err
	}
	for _, vm := range vms {
		if vm.IsStarting() || vm.State() == hypervctl.Enabled {
			return true, vm.ElementName, nil
		}
	}
	return false, "", nil
}

func (v HyperVVirtualization) IsValidVMName(name string) (bool, error) {
	var found bool
	vms, err := v.loadFromLocalJson()
	if err != nil {
		return false, err
	}
	for _, vm := range vms {
		if vm.Name == name {
			found = true
			break
		}
	}
	if !found {
		return false, nil
	}
	if _, err := hypervctl.NewVirtualMachineManager().GetMachine(name); err != nil {
		return false, err
	}
	return true, nil
}

func (v HyperVVirtualization) List(opts machine.ListOptions) ([]*machine.ListResponse, error) {
	mms, err := v.loadFromLocalJson()
	if err != nil {
		return nil, err
	}

	var response []*machine.ListResponse
	vmm := hypervctl.NewVirtualMachineManager()

	for _, mm := range mms {
		vm, err := vmm.GetMachine(mm.Name)
		if err != nil {
			return nil, err
		}
		mlr := machine.ListResponse{
			Name:           mm.Name,
			CreatedAt:      mm.Created,
			LastUp:         mm.LastUp,
			Running:        vm.State() == hypervctl.Enabled,
			Starting:       mm.isStarting(),
			Stream:         mm.ImageStream,
			VMType:         machine.HyperVVirt.String(),
			CPUs:           mm.CPUs,
			Memory:         mm.Memory * units.MiB,
			DiskSize:       mm.DiskSize * units.GiB,
			Port:           mm.Port,
			RemoteUsername: mm.RemoteUsername,
			IdentityPath:   mm.IdentityPath,
		}
		response = append(response, &mlr)
	}
	return response, err
}

func (v HyperVVirtualization) LoadVMByName(name string) (machine.VM, error) {
	m := &HyperVMachine{Name: name}
	return m.loadFromFile()
}

func (v HyperVVirtualization) NewMachine(opts machine.InitOptions) (machine.VM, error) {
	m := HyperVMachine{Name: opts.Name}
	if len(opts.ImagePath) < 1 {
		return nil, errors.New("must define --image-path for hyperv support")
	}

	m.RemoteUsername = opts.Username

	configDir, err := machine.GetConfDir(machine.HyperVVirt)
	if err != nil {
		return nil, err
	}

	configPath, err := machine.NewMachineFile(getVMConfigPath(configDir, opts.Name), nil)
	if err != nil {
		return nil, err
	}

	m.ConfigPath = *configPath

	ignitionPath, err := machine.NewMachineFile(filepath.Join(configDir, m.Name)+".ign", nil)
	if err != nil {
		return nil, err
	}
	m.IgnitionFile = *ignitionPath

	// Set creation time
	m.Created = time.Now()

	dataDir, err := machine.GetDataDir(machine.HyperVVirt)
	if err != nil {
		return nil, err
	}

	// Set the proxy pid file
	gvProxyPid, err := machine.NewMachineFile(filepath.Join(dataDir, "gvproxy.pid"), nil)
	if err != nil {
		return nil, err
	}
	m.GvProxyPid = *gvProxyPid

	dl, err := VirtualizationProvider().NewDownload(m.Name)
	if err != nil {
		return nil, err
	}
	// Acquire the image
	imagePath, imageStream, err := dl.AcquireVMImage(opts.ImagePath, opts.GuestOS)
	if err != nil {
		return nil, err
	}

	// assign values to machine
	m.ImagePath = *imagePath
	m.ImageStream = imageStream.String()

	config := hypervctl.HardwareConfig{
		CPUs:     uint16(opts.CPUS),
		DiskPath: imagePath.GetPath(),
		DiskSize: opts.DiskSize,
		Memory:   opts.Memory,
	}

	// Write the json configuration file which will be loaded by
	// LoadByName
	b, err := json.MarshalIndent(m, "", " ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(m.ConfigPath.GetPath(), b, 0644); err != nil {
		return nil, err
	}

	vmm := hypervctl.NewVirtualMachineManager()
	if err := vmm.NewVirtualMachine(opts.Name, &config); err != nil {
		return nil, err
	}
	return v.LoadVMByName(opts.Name)
}

func (v HyperVVirtualization) RemoveAndCleanMachines() error {
	// Error handling used here is following what qemu did
	var (
		prevErr error
	)

	// The next three info lookups must succeed or we return
	mms, err := v.loadFromLocalJson()
	if err != nil {
		return err
	}

	configDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return err
	}

	dataDir, err := machine.GetDataDir(vmtype)
	if err != nil {
		return err
	}

	vmm := hypervctl.NewVirtualMachineManager()
	for _, mm := range mms {
		vm, err := vmm.GetMachine(mm.Name)
		if err != nil {
			prevErr = handlePrevError(err, prevErr)
		}

		if vm.State() != hypervctl.Disabled {
			if err := vm.StopWithForce(); err != nil {
				prevErr = handlePrevError(err, prevErr)
			}
		}
		if err := vm.Remove(mm.ImagePath.GetPath()); err != nil {
			prevErr = handlePrevError(err, prevErr)
		}
		if err := mm.ReadyHVSock.Remove(); err != nil {
			prevErr = handlePrevError(err, prevErr)
		}
		if err := mm.NetworkHVSock.Remove(); err != nil {
			prevErr = handlePrevError(err, prevErr)
		}
	}

	// Nuke the config and dataDirs
	if err := os.RemoveAll(configDir); err != nil {
		prevErr = handlePrevError(err, prevErr)
	}
	if err := os.RemoveAll(dataDir); err != nil {
		prevErr = handlePrevError(err, prevErr)
	}
	return prevErr
}

func (v HyperVVirtualization) VMType() machine.VMType {
	return vmtype
}

func (v HyperVVirtualization) loadFromLocalJson() ([]*HyperVMachine, error) {
	var (
		jsonFiles []string
		mms       []*HyperVMachine
	)
	configDir, err := machine.GetConfDir(v.VMType())
	if err != nil {
		return nil, err
	}
	if err := filepath.WalkDir(configDir, func(input string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if filepath.Ext(d.Name()) == ".json" {
			jsonFiles = append(jsonFiles, input)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	for _, jsonFile := range jsonFiles {
		mm := HyperVMachine{}
		if err := mm.loadHyperVMachineFromJSON(jsonFile); err != nil {
			return nil, err
		}
		if err != nil {
			return nil, err
		}
		mms = append(mms, &mm)
	}
	return mms, nil
}

func handlePrevError(e, prevErr error) error {
	if prevErr != nil {
		logrus.Error(e)
	}
	return e
}

func stateConversion(s hypervctl.EnabledState) (machine.Status, error) {
	switch s {
	case hypervctl.Enabled:
		return machine.Running, nil
	case hypervctl.Disabled:
		return machine.Stopped, nil
	case hypervctl.Starting:
		return machine.Starting, nil
	}
	return machine.Unknown, fmt.Errorf("unknown state: %q", s.String())
}
