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
	"github.com/containers/podman/v4/pkg/machine/compression"
	"github.com/containers/podman/v4/pkg/machine/define"
	"github.com/containers/podman/v4/utils"
	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
)

type QEMUVirtualization struct {
	machine.Virtualization
}

// setNewMachineCMDOpts are options needed to pass
// into setting up the qemu command line.  long term, this need
// should be eliminated
// TODO Podman5
type setNewMachineCMDOpts struct {
	imageDir string
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
func (v *MachineVM) setNewMachineCMD(qemuBinary string, cmdOpts *setNewMachineCMDOpts) {
	v.CmdLine = NewQemuBuilder(qemuBinary, v.addArchOptions(cmdOpts))
	v.CmdLine.SetMemory(v.Memory)
	v.CmdLine.SetCPUs(v.CPUs)
	v.CmdLine.SetIgnitionFile(v.IgnitionFile)
	v.CmdLine.SetQmpMonitor(v.QMPMonitor)
	v.CmdLine.SetNetwork()
	v.CmdLine.SetSerialPort(v.ReadySocket, v.VMPidFilePath, v.Name)
	v.CmdLine.SetUSBHostPassthrough(v.USBs)
}

func parseUSBs(usbs []string) ([]machine.USBConfig, error) {
	configs := []machine.USBConfig{}
	for _, str := range usbs {
		if str == "" {
			// Ignore --usb="" as it can be used to reset USBConfigs
			continue
		}

		vals := strings.Split(str, ",")
		if len(vals) != 2 {
			return configs, fmt.Errorf("usb: fail to parse: missing ',': %s", str)
		}

		left := strings.Split(vals[0], "=")
		if len(left) != 2 {
			return configs, fmt.Errorf("usb: fail to parse: missing '=': %s", str)
		}

		right := strings.Split(vals[1], "=")
		if len(right) != 2 {
			return configs, fmt.Errorf("usb: fail to parse: missing '=': %s", str)
		}

		option := ""
		if (left[0] == "bus" && right[0] == "devnum") ||
			(right[0] == "bus" && left[0] == "devnum") {
			option = "bus_devnum"
		}
		if (left[0] == "vendor" && right[0] == "product") ||
			(right[0] == "vendor" && left[0] == "product") {
			option = "vendor_product"
		}

		switch option {
		case "bus_devnum":
			bus, devnumber := left[1], right[1]
			if right[0] == "bus" {
				bus, devnumber = devnumber, bus
			}

			configs = append(configs, machine.USBConfig{
				Bus:       bus,
				DevNumber: devnumber,
			})
		case "vendor_product":
			vendorStr, productStr := left[1], right[1]
			if right[0] == "vendor" {
				vendorStr, productStr = productStr, vendorStr
			}

			vendor, err := strconv.ParseInt(vendorStr, 16, 0)
			if err != nil {
				return configs, fmt.Errorf("fail to convert vendor of %s: %s", str, err)
			}

			product, err := strconv.ParseInt(productStr, 16, 0)
			if err != nil {
				return configs, fmt.Errorf("fail to convert product of %s: %s", str, err)
			}

			configs = append(configs, machine.USBConfig{
				Vendor:  int(vendor),
				Product: int(product),
			})
		default:
			return configs, fmt.Errorf("usb: fail to parse: %s", str)
		}
	}
	return configs, nil
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

	dataDir, err := machine.GetDataDir(p.VMType())
	if err != nil {
		return nil, err
	}

	// set VM ignition file
	ignitionFile, err := define.NewMachineFile(filepath.Join(vmConfigDir, vm.Name+".ign"), nil)
	if err != nil {
		return nil, err
	}
	vm.IgnitionFile = *ignitionFile

	// set VM image file
	imagePath, err := define.NewMachineFile(opts.ImagePath, nil)
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
	if vm.USBs, err = parseUSBs(opts.USBs); err != nil {
		return nil, err
	}

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

	runtimeDir, err := getRuntimeDir()
	if err != nil {
		return nil, err
	}
	symlink := vm.Name + "_ready.sock"
	if err := machine.SetSocket(&vm.ReadySocket, machine.ReadySocketPath(runtimeDir+"/podman/", vm.Name), &symlink); err != nil {
		return nil, err
	}

	// configure command to run
	cmdOpts := setNewMachineCMDOpts{imageDir: dataDir}
	vm.setNewMachineCMD(execPath, &cmdOpts)
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

	lock, err := machine.GetLock(vm.Name, vmtype)
	if err != nil {
		return nil, err
	}
	vm.lock = lock

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
		err := utils.GuardedRemoveAll(dataDir)
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
		err := utils.GuardedRemoveAll(confDir)
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
		machine.NewVirtualization(define.Qemu, compression.Xz, define.Qcow, vmtype),
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
