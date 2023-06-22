//go:build arm64 && darwin

package applehv

import (
	"errors"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/containers/podman/v4/pkg/machine"
	"github.com/docker/go-units"
	"golang.org/x/sys/unix"
)

const (
	defaultVFKitEndpoint = "http://localhost:8081"
)

type AppleHVVirtualization struct {
	machine.Virtualization
}

type MMHardwareConfig struct {
	CPUs     uint16
	DiskPath string
	DiskSize uint64
	Memory   int32
}

func VirtualizationProvider() machine.VirtProvider {
	return &AppleHVVirtualization{
		machine.NewVirtualization(machine.Metal, machine.Xz, machine.Raw),
	}
}

func (v AppleHVVirtualization) CheckExclusiveActiveVM() (bool, string, error) {
	fsVms, err := getVMInfos()
	if err != nil {
		return false, "", err
	}
	for _, vm := range fsVms {
		if vm.Running || vm.Starting {
			return true, vm.Name, nil
		}
	}

	return false, "", nil
}

func (v AppleHVVirtualization) IsValidVMName(name string) (bool, error) {
	mm := MacMachine{Name: name}
	configDir, err := machine.GetConfDir(machine.AppleHvVirt)
	if err != nil {
		return false, err
	}
	if err := loadMacMachineFromJSON(configDir, &mm); err != nil {
		return false, err
	}
	return true, nil
}

func (v AppleHVVirtualization) List(opts machine.ListOptions) ([]*machine.ListResponse, error) {
	var (
		response []*machine.ListResponse
	)

	mms, err := v.loadFromLocalJson()
	if err != nil {
		return nil, err
	}

	for _, mm := range mms {
		vmState, err := mm.state()
		if err != nil {
			if errors.Is(err, unix.ECONNREFUSED) {
				vmState = machine.Stopped
			} else {
				return nil, err
			}
		}

		mlr := machine.ListResponse{
			Name:           mm.Name,
			CreatedAt:      mm.Created,
			LastUp:         mm.LastUp,
			Running:        vmState == machine.Running,
			Starting:       vmState == machine.Starting,
			Stream:         mm.ImageStream,
			VMType:         machine.AppleHvVirt.String(),
			CPUs:           mm.CPUs,
			Memory:         mm.Memory * units.MiB,
			DiskSize:       mm.DiskSize * units.GiB,
			Port:           mm.Port,
			RemoteUsername: mm.RemoteUsername,
			IdentityPath:   mm.IdentityPath,
		}
		response = append(response, &mlr)
	}
	return response, nil
}

func (v AppleHVVirtualization) LoadVMByName(name string) (machine.VM, error) {
	m := MacMachine{Name: name}
	return m.loadFromFile()
}

func (v AppleHVVirtualization) NewMachine(opts machine.InitOptions) (machine.VM, error) {
	m := MacMachine{Name: opts.Name}

	configDir, err := machine.GetConfDir(machine.AppleHvVirt)
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

	m.ResourceConfig = machine.ResourceConfig{
		CPUs:     opts.CPUS,
		DiskSize: opts.DiskSize,
		// Diskpath will be needed
		Memory: opts.Memory,
	}

	if err := m.writeConfig(); err != nil {
		return nil, err
	}
	return m.loadFromFile()
}

func (v AppleHVVirtualization) RemoveAndCleanMachines() error {
	// This can be implemented when host networking is completed.
	return machine.ErrNotImplemented
}

func (v AppleHVVirtualization) VMType() machine.VMType {
	return vmtype
}

func (v AppleHVVirtualization) loadFromLocalJson() ([]*MacMachine, error) {
	var (
		jsonFiles []string
		mms       []*MacMachine
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
		mm := MacMachine{}
		if err := loadMacMachineFromJSON(jsonFile, &mm); err != nil {
			return nil, err
		}
		if err != nil {
			return nil, err
		}
		mms = append(mms, &mm)
	}
	return mms, nil
}
