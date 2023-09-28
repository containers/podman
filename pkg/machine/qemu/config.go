package qemu

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/utils"
	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
)

type QEMUVirtualization struct {
	machine.Virtualization
}

// findQEMUBinary locates and returns the QEMU binary
func findQEMUBinary() (string, error) {
	cfg, err := config.Default()
	if err != nil {
		return "", err
	}
	return cfg.FindHelperBinary(QemuCommand, true)
}

// setQMPMonitorSocket sets the virtual machine's QMP Monitor socket
func (v *MachineVM) setQMPMonitorSocket() error {
	monitor, err := NewQMPMonitor("unix", v.Name, defaultQMPTimeout)
	if err != nil {
		return err
	}
	v.QMPMonitor = monitor
	return nil
}

// setNewMachineCMD configure the CLI command that will be run to create the new
// machine
func (v *MachineVM) setNewMachineCMD(qemuBinary string) {
	cmd := []string{qemuBinary}
	// Add memory
	cmd = append(cmd, []string{"-m", strconv.Itoa(int(v.Memory))}...)
	// Add cpus
	cmd = append(cmd, []string{"-smp", strconv.Itoa(int(v.CPUs))}...)
	// Add ignition file
	cmd = append(cmd, []string{"-fw_cfg", "name=opt/com.coreos/config,file=" + v.IgnitionFile.GetPath()}...)
	cmd = append(cmd, []string{"-qmp", v.QMPMonitor.Network + ":" + v.QMPMonitor.Address.GetPath() + ",server=on,wait=off"}...)

	// Add network
	// Right now the mac address is hardcoded so that the host networking gives it a specific IP address.  This is
	// why we can only run one vm at a time right now
	cmd = append(cmd, []string{"-netdev", "socket,id=vlan,fd=3", "-device", "virtio-net-pci,netdev=vlan,mac=5a:94:ef:e4:0c:ee"}...)

	// Add serial port for readiness
	cmd = append(cmd, []string{
		"-device", "virtio-serial",
		// qemu needs to establish the long name; other connections can use the symlink'd
		// Note both id and chardev start with an extra "a" because qemu requires that it
		// starts with a letter but users can also use numbers
		"-chardev", "socket,path=" + v.ReadySocket.Path + ",server=on,wait=off,id=a" + v.Name + "_ready",
		"-device", "virtserialport,chardev=a" + v.Name + "_ready" + ",name=org.fedoraproject.port.0",
		"-pidfile", v.VMPidFilePath.GetPath()}...)
	v.CmdLine = cmd
}

// NewMachine initializes an instance of a virtual machine based on the qemu
// virtualization.
func (p *QEMUVirtualization) NewMachine(opts machine.InitOptions) (machine.VM, error) {
	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return nil, err
	}
	vm := new(MachineVM)
	if len(opts.Name) > 0 {
		vm.Name = opts.Name
	}

	// set VM ignition file
	ignitionFile, err := machine.NewMachineFile(filepath.Join(vmConfigDir, vm.Name+".ign"), nil)
	if err != nil {
		return nil, err
	}
	vm.IgnitionFile = *ignitionFile

	// set VM image file
	imagePath, err := machine.NewMachineFile(opts.ImagePath, nil)
	if err != nil {
		return nil, err
	}
	vm.ImagePath = *imagePath

	vm.RemoteUsername = opts.Username

	// Add a random port for ssh
	port, err := utils.GetRandomPort()
	if err != nil {
		return nil, err
	}
	vm.Port = port

	vm.CPUs = opts.CPUS
	vm.Memory = opts.Memory
	vm.DiskSize = opts.DiskSize

	vm.Created = time.Now()

	// find QEMU binary
	execPath, err := findQEMUBinary()
	if err != nil {
		return nil, err
	}

	if err := vm.setPIDSocket(); err != nil {
		return nil, err
	}

	// Add qmp socket
	if err := vm.setQMPMonitorSocket(); err != nil {
		return nil, err
	}

	if err := vm.setReadySocket(); err != nil {
		return nil, err
	}

	// configure command to run
	vm.setNewMachineCMD(execPath)
	return vm, nil
}

// LoadVMByName reads a json file that describes a known qemu vm
// and returns a vm instance
func (p *QEMUVirtualization) LoadVMByName(name string) (machine.VM, error) {
	vm := &MachineVM{Name: name}
	vm.HostUser = machine.HostUser{UID: -1} // posix reserves -1, so use it to signify undefined
	if err := vm.update(); err != nil {
		return nil, err
	}

	return vm, nil
}

// List lists all vm's that use qemu virtualization
func (p *QEMUVirtualization) List(_ machine.ListOptions) ([]*machine.ListResponse, error) {
	return getVMInfos()
}

func getVMInfos() ([]*machine.ListResponse, error) {
	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return nil, err
	}

	var listed []*machine.ListResponse

	if err = filepath.WalkDir(vmConfigDir, func(path string, d fs.DirEntry, err error) error {
		vm := new(MachineVM)
		if strings.HasSuffix(d.Name(), ".json") {
			fullPath := filepath.Join(vmConfigDir, d.Name())
			b, err := os.ReadFile(fullPath)
			if err != nil {
				return err
			}
			err = json.Unmarshal(b, vm)
			if err != nil {
				// Checking if the file did not unmarshal because it is using
				// the deprecated config file format.
				migrateErr := migrateVM(fullPath, b, vm)
				if migrateErr != nil {
					return migrateErr
				}
			}
			listEntry := new(machine.ListResponse)

			listEntry.Name = vm.Name
			listEntry.Stream = vm.ImageStream
			listEntry.VMType = "qemu"
			listEntry.CPUs = vm.CPUs
			listEntry.Memory = vm.Memory * units.MiB
			listEntry.DiskSize = vm.DiskSize * units.GiB
			listEntry.Port = vm.Port
			listEntry.RemoteUsername = vm.RemoteUsername
			listEntry.IdentityPath = vm.IdentityPath
			listEntry.CreatedAt = vm.Created
			listEntry.Starting = vm.Starting
			listEntry.UserModeNetworking = true // always true

			if listEntry.CreatedAt.IsZero() {
				listEntry.CreatedAt = time.Now()
				vm.Created = time.Now()
				if err := vm.writeConfig(); err != nil {
					return err
				}
			}

			state, err := vm.State(false)
			if err != nil {
				return err
			}
			listEntry.Running = state == machine.Running
			listEntry.LastUp = vm.LastUp

			listed = append(listed, listEntry)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return listed, err
}

func (p *QEMUVirtualization) IsValidVMName(name string) (bool, error) {
	infos, err := getVMInfos()
	if err != nil {
		return false, err
	}
	for _, vm := range infos {
		if vm.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// CheckExclusiveActiveVM checks if there is a VM already running
// that does not allow other VMs to be running
func (p *QEMUVirtualization) CheckExclusiveActiveVM() (bool, string, error) {
	vms, err := getVMInfos()
	if err != nil {
		return false, "", fmt.Errorf("checking VM active: %w", err)
	}
	// NOTE: Start() takes care of dealing with the "starting" state.
	for _, vm := range vms {
		if vm.Running {
			return true, vm.Name, nil
		}
	}
	return false, "", nil
}

// RemoveAndCleanMachines removes all machine and cleans up any other files associated with podman machine
func (p *QEMUVirtualization) RemoveAndCleanMachines() error {
	var (
		vm             machine.VM
		listResponse   []*machine.ListResponse
		opts           machine.ListOptions
		destroyOptions machine.RemoveOptions
	)
	destroyOptions.Force = true
	var prevErr error

	listResponse, err := p.List(opts)
	if err != nil {
		return err
	}

	for _, mach := range listResponse {
		vm, err = p.LoadVMByName(mach.Name)
		if err != nil {
			if prevErr != nil {
				logrus.Error(prevErr)
			}
			prevErr = err
		}
		_, remove, err := vm.Remove(mach.Name, destroyOptions)
		if err != nil {
			if prevErr != nil {
				logrus.Error(prevErr)
			}
			prevErr = err
		} else {
			if err := remove(); err != nil {
				if prevErr != nil {
					logrus.Error(prevErr)
				}
				prevErr = err
			}
		}
	}

	// Clean leftover files in data dir
	dataDir, err := machine.DataDirPrefix()
	if err != nil {
		if prevErr != nil {
			logrus.Error(prevErr)
		}
		prevErr = err
	} else {
		err := machine.GuardedRemoveAll(dataDir)
		if err != nil {
			if prevErr != nil {
				logrus.Error(prevErr)
			}
			prevErr = err
		}
	}

	// Clean leftover files in conf dir
	confDir, err := machine.ConfDirPrefix()
	if err != nil {
		if prevErr != nil {
			logrus.Error(prevErr)
		}
		prevErr = err
	} else {
		err := machine.GuardedRemoveAll(confDir)
		if err != nil {
			if prevErr != nil {
				logrus.Error(prevErr)
			}
			prevErr = err
		}
	}
	return prevErr
}

func (p *QEMUVirtualization) VMType() machine.VMType {
	return vmtype
}

func VirtualizationProvider() machine.VirtProvider {
	return &QEMUVirtualization{
		machine.NewVirtualization(machine.Qemu, machine.Xz, machine.Qcow, vmtype),
	}
}

// Deprecated: MachineVMV1 is being deprecated in favor a more flexible and informative
// structure
type MachineVMV1 struct {
	// CPUs to be assigned to the VM
	CPUs uint64
	// The command line representation of the qemu command
	CmdLine []string
	// Mounts is the list of remote filesystems to mount
	Mounts []machine.Mount
	// IdentityPath is the fq path to the ssh priv key
	IdentityPath string
	// IgnitionFilePath is the fq path to the .ign file
	IgnitionFilePath string
	// ImageStream is the update stream for the image
	ImageStream string
	// ImagePath is the fq path to
	ImagePath string
	// Memory in megabytes assigned to the vm
	Memory uint64
	// Disk size in gigabytes assigned to the vm
	DiskSize uint64
	// Name of the vm
	Name string
	// SSH port for user networking
	Port int
	// QMPMonitor is the qemu monitor object for sending commands
	QMPMonitor Monitorv1
	// RemoteUsername of the vm user
	RemoteUsername string
	// Whether this machine should run in a rootful or rootless manner
	Rootful bool
	// UID is the numerical id of the user that called machine
	UID int
}

type Monitorv1 struct {
	//	Address portion of the qmp monitor (/tmp/tmp.sock)
	Address string
	// Network portion of the qmp monitor (unix)
	Network string
	// Timeout in seconds for qmp monitor transactions
	Timeout time.Duration
}

var (
	// defaultQMPTimeout is the timeout duration for the
	// qmp monitor interactions.
	defaultQMPTimeout = 2 * time.Second
)
