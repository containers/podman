//go:build darwin

package applehv

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"path/filepath"
	"time"

	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/compression"
	"github.com/containers/podman/v4/pkg/machine/define"
	"github.com/containers/podman/v4/pkg/machine/ignition"
	"github.com/containers/podman/v4/pkg/machine/sockets"
	"github.com/containers/podman/v4/pkg/machine/vmconfigs"
	vfConfig "github.com/crc-org/vfkit/pkg/config"
	"github.com/docker/go-units"
	"golang.org/x/sys/unix"
)

const (
	localhostURI       = "http://localhost"
	ignitionSocketName = "ignition.sock"
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
		machine.NewVirtualization(define.AppleHV, compression.Xz, define.Raw, vmtype),
	}
}

func (v AppleHVVirtualization) DeleteReadySock(sock interface{}) error {
	return sock.(*define.VMFile).Delete()
}

func (v AppleHVVirtualization) ListenReadySock(path interface{}, args ...interface{}) (interface{}, error) {
	return net.Listen("unix", path.(string))
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
	configDir, err := machine.GetConfDir(define.AppleHvVirt)
	if err != nil {
		return false, err
	}
	fqName := filepath.Join(configDir, fmt.Sprintf("%s.json", name))
	if _, err := loadMacMachineFromJSON(fqName); err != nil {
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
		vmState, err := mm.Vfkit.State()
		if err != nil {
			if errors.Is(err, unix.ECONNREFUSED) {
				vmState = define.Stopped
			} else {
				return nil, err
			}
		}

		mlr := machine.ListResponse{
			Name:           mm.Name,
			CreatedAt:      mm.Created,
			LastUp:         mm.LastUp,
			Running:        vmState == define.Running,
			Starting:       vmState == define.Starting,
			Stream:         mm.ImageStream,
			VMType:         define.AppleHvVirt.String(),
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

func (v AppleHVVirtualization) CreateReadySock(loc interface{}, name, path string) error {
	return sockets.SetSocket(loc.(*define.VMFile), sockets.ReadySocketPath(path+"/podman/", name), nil)
}

func (v AppleHVVirtualization) NewMachine(opts machine.InitOptions) (machine.VM, error) {
	m := MacMachine{Name: opts.Name}

	if len(opts.USBs) > 0 {
		return nil, fmt.Errorf("USB host passthrough is not supported for applehv machines")
	}

	configDir, err := machine.GetConfDir(define.AppleHvVirt)
	if err != nil {
		return nil, err
	}

	configPath, err := define.NewMachineFile(getVMConfigPath(configDir, opts.Name), nil)
	if err != nil {
		return nil, err
	}
	m.ConfigPath = *configPath

	dataDir, err := machine.GetDataDir(define.AppleHvVirt)
	if err != nil {
		return nil, err
	}

	if err := ignition.SetIgnitionFile(&m.IgnitionFile, vmtype, m.Name, configDir); err != nil {
		return nil, err
	}

	// Set creation time
	m.Created = time.Now()

	m.ResourceConfig = vmconfigs.ResourceConfig{
		CPUs:     opts.CPUS,
		DiskSize: opts.DiskSize,
		// Diskpath will be needed
		Memory: opts.Memory,
	}
	bl := vfConfig.NewEFIBootloader(fmt.Sprintf("%s/%ss", dataDir, opts.Name), true)
	m.Vfkit.VirtualMachine = vfConfig.NewVirtualMachine(uint(opts.CPUS), opts.Memory, bl)

	if err := m.writeConfig(); err != nil {
		return nil, err
	}
	return m.loadFromFile()
}

func (v AppleHVVirtualization) RemoveAndCleanMachines() error {
	// This can be implemented when host networking is completed.
	return machine.ErrNotImplemented
}

func (v AppleHVVirtualization) VMType() define.VMType {
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
		mm, err := loadMacMachineFromJSON(jsonFile)
		if err != nil {
			return nil, err
		}
		if err != nil {
			return nil, err
		}
		mms = append(mms, mm)
	}
	return mms, nil
}
