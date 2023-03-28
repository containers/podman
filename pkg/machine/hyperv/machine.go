//go:build windows
// +build windows

package hyperv

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/containers/libhvee/pkg/hypervctl"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/storage/pkg/homedir"
	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
)

var (
	// vmtype refers to qemu (vs libvirt, krun, etc).
	vmtype = machine.HyperVVirt
)

func GetVirtualizationProvider() machine.VirtProvider {
	return &Virtualization{
		artifact:    machine.HyperV,
		compression: machine.Zip,
		format:      machine.Vhdx,
	}
}

const (
	// Some of this will need to change when we are closer to having
	// working code.
	VolumeTypeVirtfs     = "virtfs"
	MountType9p          = "9p"
	dockerSock           = "/var/run/docker.sock"
	dockerConnectTimeout = 5 * time.Second
	apiUpTimeout         = 20 * time.Second
)

type apiForwardingState int

const (
	noForwarding apiForwardingState = iota
	claimUnsupported
	notInstalled
	machineLocal
	dockerGlobal
)

type HyperVMachine struct {
	// copied from qemu, cull and add as needed

	// ConfigPath is the fully qualified path to the configuration file
	ConfigPath machine.VMFile
	// The command line representation of the qemu command
	//CmdLine []string
	// HostUser contains info about host user
	machine.HostUser
	// ImageConfig describes the bootable image
	machine.ImageConfig
	// Mounts is the list of remote filesystems to mount
	Mounts []machine.Mount
	// Name of VM
	Name string
	// PidFilePath is the where the Proxy PID file lives
	//PidFilePath machine.VMFile
	// VMPidFilePath is the where the VM PID file lives
	//VMPidFilePath machine.VMFile
	// QMPMonitor is the qemu monitor object for sending commands
	//QMPMonitor Monitor
	// ReadySocket tells host when vm is booted
	ReadySocket machine.VMFile
	// ResourceConfig is physical attrs of the VM
	machine.ResourceConfig
	// SSHConfig for accessing the remote vm
	machine.SSHConfig
	// Starting tells us whether the machine is running or if we have just dialed it to start it
	Starting bool
	// Created contains the original created time instead of querying the file mod time
	Created time.Time
	// LastUp contains the last recorded uptime
	LastUp time.Time
}

func (m *HyperVMachine) Init(opts machine.InitOptions) (bool, error) {
	var (
		key string
	)

	sshDir := filepath.Join(homedir.Get(), ".ssh")
	m.IdentityPath = filepath.Join(sshDir, m.Name)

	if len(opts.IgnitionPath) < 1 {
		uri := machine.SSHRemoteConnection.MakeSSHURL("localhost", fmt.Sprintf("/run/user/%d/podman/podman.sock", m.UID), strconv.Itoa(m.Port), m.RemoteUsername)
		uriRoot := machine.SSHRemoteConnection.MakeSSHURL("localhost", "/run/podman/podman.sock", strconv.Itoa(m.Port), "root")
		identity := filepath.Join(sshDir, m.Name)

		uris := []url.URL{uri, uriRoot}
		names := []string{m.Name, m.Name + "-root"}

		// The first connection defined when connections is empty will become the default
		// regardless of IsDefault, so order according to rootful
		if opts.Rootful {
			uris[0], names[0], uris[1], names[1] = uris[1], names[1], uris[0], names[0]
		}

		for i := 0; i < 2; i++ {
			if err := machine.AddConnection(&uris[i], names[i], identity, opts.IsDefault && i == 0); err != nil {
				return false, err
			}
		}
	} else {
		fmt.Println("An ignition path was provided.  No SSH connection was added to Podman")
	}
	if len(opts.IgnitionPath) < 1 {
		var err error
		key, err = machine.CreateSSHKeys(m.IdentityPath)
		if err != nil {
			return false, err
		}
	}

	m.ResourceConfig = machine.ResourceConfig{
		CPUs:     opts.CPUS,
		DiskSize: opts.DiskSize,
		Memory:   opts.Memory,
	}

	// If the user provides an ignition file, we need to
	// copy it into the conf dir
	if len(opts.IgnitionPath) > 0 {
		inputIgnition, err := os.ReadFile(opts.IgnitionPath)
		if err != nil {
			return false, err
		}
		return false, os.WriteFile(m.IgnitionFile.GetPath(), inputIgnition, 0644)
	}

	// Write the JSON file for the second time.  First time was in NewMachine
	b, err := json.MarshalIndent(m, "", " ")
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(m.ConfigPath.GetPath(), b, 0644); err != nil {
		return false, err
	}

	if m.UID == 0 {
		m.UID = 1000
	}

	// c/common sets the default machine user for "windows" to be "user"; this
	// is meant for the WSL implementation that does not use FCOS.  For FCOS,
	// however, we want to use the DefaultIgnitionUserName which is currently
	// "core"
	user := opts.Username
	if user == "user" {
		user = machine.DefaultIgnitionUserName
	}
	// Write the ignition file
	ign := machine.DynamicIgnition{
		Name:      user,
		Key:       key,
		VMName:    m.Name,
		TimeZone:  opts.TimeZone,
		WritePath: m.IgnitionFile.GetPath(),
		UID:       m.UID,
	}

	if err := machine.NewIgnitionFile(ign, machine.HyperVVirt); err != nil {
		return false, err
	}
	// The ignition file has been written. We now need to
	// read it so that we can put it into key-value pairs
	ignFile, err := m.IgnitionFile.Read()
	if err != nil {
		return false, err
	}
	reader := bytes.NewReader(ignFile)

	vm, err := hypervctl.NewVirtualMachineManager().GetMachine(m.Name)
	if err != nil {
		return false, err
	}
	err = vm.SplitAndAddIgnition("ignition.config.", reader)
	return err == nil, err
}

func (m *HyperVMachine) Inspect() (*machine.InspectInfo, error) {
	vm, err := hypervctl.NewVirtualMachineManager().GetMachine(m.Name)
	if err != nil {
		return nil, err
	}

	cfg, err := vm.GetConfig(m.ImagePath.GetPath())
	if err != nil {
		return nil, err
	}

	return &machine.InspectInfo{
		ConfigPath:     m.ConfigPath,
		ConnectionInfo: machine.ConnectionConfig{},
		Created:        m.Created,
		Image: machine.ImageConfig{
			IgnitionFile: machine.VMFile{},
			ImageStream:  "",
			ImagePath:    machine.VMFile{},
		},
		LastUp: m.LastUp,
		Name:   m.Name,
		Resources: machine.ResourceConfig{
			CPUs:     uint64(cfg.Hardware.CPUs),
			DiskSize: 0,
			Memory:   uint64(cfg.Hardware.Memory),
		},
		SSHConfig: m.SSHConfig,
		State:     vm.State().String(),
	}, nil
}

func (m *HyperVMachine) Remove(_ string, opts machine.RemoveOptions) (string, func() error, error) {
	var (
		files    []string
		diskPath string
	)
	vmm := hypervctl.NewVirtualMachineManager()
	vm, err := vmm.GetMachine(m.Name)
	if err != nil {
		return "", nil, err
	}
	// In hyperv, they call running 'enabled'
	if vm.State() == hypervctl.Enabled {
		if !opts.Force {
			return "", nil, hypervctl.ErrMachineStateInvalid
		}
		if err := vm.Stop(); err != nil {
			return "", nil, err
		}
	}

	// Collect all the files that need to be destroyed
	if !opts.SaveKeys {
		files = append(files, m.IdentityPath, m.IdentityPath+".pub")
	}
	if !opts.SaveIgnition {
		files = append(files, m.IgnitionFile.GetPath())
	}

	if !opts.SaveImage {
		diskPath := m.ImagePath.GetPath()
		files = append(files, diskPath)
	}

	if err := machine.RemoveConnection(m.Name); err != nil {
		logrus.Error(err)
	}
	if err := machine.RemoveConnection(m.Name + "-root"); err != nil {
		logrus.Error(err)
	}
	files = append(files, getVMConfigPath(m.ConfigPath.GetPath(), m.Name))
	confirmationMessage := "\nThe following files will be deleted:\n\n"
	for _, msg := range files {
		confirmationMessage += msg + "\n"
	}

	confirmationMessage += "\n"
	return confirmationMessage, func() error {
		for _, f := range files {
			if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
				logrus.Error(err)
			}
		}
		return vm.Remove(diskPath)
	}, nil
}

func (m *HyperVMachine) Set(name string, opts machine.SetOptions) ([]error, error) {
	var (
		cpuChanged, memoryChanged bool
		setErrors                 []error
	)
	vmm := hypervctl.NewVirtualMachineManager()
	// Considering this a hard return if we cannot lookup the machine
	vm, err := vmm.GetMachine(m.Name)
	if err != nil {
		return setErrors, err
	}
	if vm.State() != hypervctl.Disabled {
		return nil, errors.New("unable to change settings unless vm is stopped")
	}

	if opts.Rootful != nil && m.Rootful != *opts.Rootful {
		setErrors = append(setErrors, hypervctl.ErrNotImplemented)
	}
	if opts.DiskSize != nil && m.DiskSize != *opts.DiskSize {
		setErrors = append(setErrors, hypervctl.ErrNotImplemented)
	}
	if opts.CPUs != nil && m.CPUs != *opts.CPUs {
		m.CPUs = *opts.CPUs
		cpuChanged = true
	}
	if opts.Memory != nil && m.Memory != *opts.Memory {
		m.Memory = *opts.Memory
		memoryChanged = true
	}

	if !cpuChanged && !memoryChanged {
		switch len(setErrors) {
		case 0:
			return nil, nil
		case 1:
			return nil, setErrors[0]
		default:
			return setErrors[1:], setErrors[0]
		}
	}
	// Write the new JSON out
	// considering this a hard return if we cannot write the JSON file.
	b, err := json.MarshalIndent(m, "", " ")
	if err != nil {
		return setErrors, err
	}
	if err := os.WriteFile(m.ConfigPath.GetPath(), b, 0644); err != nil {
		return setErrors, err
	}

	return setErrors, vm.UpdateProcessorMemSettings(func(ps *hypervctl.ProcessorSettings) {
		if cpuChanged {
			ps.VirtualQuantity = m.CPUs
		}
	}, func(ms *hypervctl.MemorySettings) {
		if memoryChanged {
			ms.DynamicMemoryEnabled = false
			ms.VirtualQuantity = m.Memory
			ms.Limit = m.Memory
			ms.Reservation = m.Memory
		}
	})
}

func (m *HyperVMachine) SSH(name string, opts machine.SSHOptions) error {
	return machine.ErrNotImplemented
}

func (m *HyperVMachine) Start(name string, opts machine.StartOptions) error {
	// TODO We need to hold Start until it actually finishes booting and ignition stuff
	vmm := hypervctl.NewVirtualMachineManager()
	vm, err := vmm.GetMachine(m.Name)
	if err != nil {
		return err
	}
	if vm.State() != hypervctl.Disabled {
		return hypervctl.ErrMachineStateInvalid
	}
	return vm.Start()
}

func (m *HyperVMachine) State(_ bool) (machine.Status, error) {
	vmm := hypervctl.NewVirtualMachineManager()
	vm, err := vmm.GetMachine(m.Name)
	if err != nil {
		return "", err
	}
	if vm.IsStarting() {
		return machine.Starting, nil
	}
	if vm.State() == hypervctl.Enabled {
		return machine.Running, nil
	}
	// Following QEMU pattern here where only three
	// states seem valid
	return machine.Stopped, nil
}

func (m *HyperVMachine) Stop(name string, opts machine.StopOptions) error {
	vmm := hypervctl.NewVirtualMachineManager()
	vm, err := vmm.GetMachine(m.Name)
	if err != nil {
		return err
	}
	if vm.State() != hypervctl.Enabled {
		return hypervctl.ErrMachineStateInvalid
	}
	return vm.Stop()
}

func (m *HyperVMachine) jsonConfigPath() (string, error) {
	configDir, err := machine.GetConfDir(machine.HyperVVirt)
	if err != nil {
		return "", err
	}
	return getVMConfigPath(configDir, m.Name), nil
}

func (m *HyperVMachine) loadFromFile() (*HyperVMachine, error) {
	if len(m.Name) < 1 {
		return nil, errors.New("encountered machine with no name")
	}

	jsonPath, err := m.jsonConfigPath()
	if err != nil {
		return nil, err
	}
	mm := HyperVMachine{}

	if err := loadMacMachineFromJSON(jsonPath, &mm); err != nil {
		return nil, err
	}
	vmm := hypervctl.NewVirtualMachineManager()
	vm, err := vmm.GetMachine(m.Name)
	if err != nil {
		return nil, err
	}

	cfg, err := vm.GetConfig(mm.ImagePath.GetPath())
	if err != nil {
		return nil, err
	}

	// If the machine is on, we can get what it is actually using
	if cfg.Hardware.CPUs > 0 {
		mm.CPUs = uint64(cfg.Hardware.CPUs)
	}
	// Same for memory
	if cfg.Hardware.Memory > 0 {
		mm.Memory = uint64(cfg.Hardware.Memory)
	}

	mm.DiskSize = cfg.Hardware.DiskSize * units.MiB
	mm.LastUp = cfg.Status.LastUp

	return &mm, nil
}

// getVMConfigPath is a simple wrapper for getting the fully-qualified
// path of the vm json config file.  It should be used to get conformity
func getVMConfigPath(configDir, vmName string) string {
	return filepath.Join(configDir, fmt.Sprintf("%s.json", vmName))
}

func loadMacMachineFromJSON(fqConfigPath string, macMachine *HyperVMachine) error {
	b, err := os.ReadFile(fqConfigPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%q: %w", fqConfigPath, machine.ErrNoSuchVM)
		}
		return err
	}
	return json.Unmarshal(b, macMachine)
}
