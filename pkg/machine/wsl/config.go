//go:build tempoff

package wsl

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/compression"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/utils"
	"github.com/sirupsen/logrus"
)

type WSLVirtualization struct {
	machine.Virtualization
}

func VirtualizationProvider() machine.VirtProvider {
	return &WSLVirtualization{
		machine.NewVirtualization(define.None, compression.Xz, define.Tar, vmtype),
	}
}

// NewMachine initializes an instance of a wsl machine
func (p *WSLVirtualization) NewMachine(opts define.InitOptions) (machine.VM, error) {
	vm := new(MachineVM)
	if len(opts.USBs) > 0 {
		return nil, fmt.Errorf("USB host passthrough is not supported for WSL machines")
	}
	if len(opts.Name) > 0 {
		vm.Name = opts.Name
	}
	configPath, err := getConfigPath(opts.Name)
	if err != nil {
		return vm, err
	}

	vm.ConfigPath = configPath
	vm.ImagePath = opts.ImagePath

	// WSL historically uses a different username; translate "core" fcos default to
	// legacy "user" default
	if opts.Username == "" || opts.Username == "core" {
		vm.RemoteUsername = "user"
	} else {
		vm.RemoteUsername = opts.Username
	}

	vm.Created = time.Now()

	// Default is false
	if opts.UserModeNetworking != nil {
		vm.UserModeNetworking = *opts.UserModeNetworking
	}

	// Add a random port for ssh
	port, err := machine.AllocateMachinePort()
	if err != nil {
		return nil, err
	}
	vm.Port = port

	return vm, nil
}

// LoadByName reads a json file that describes a known qemu vm
// and returns a vm instance
func (p *WSLVirtualization) LoadVMByName(name string) (machine.VM, error) {
	configPath, err := getConfigPath(name)
	if err != nil {
		return nil, err
	}

	vm, err := readAndMigrate(configPath, name)
	if err != nil {
		return nil, err
	}

	lock, err := machine.GetLock(vm.Name, vmtype)
	if err != nil {
		return nil, err
	}
	vm.lock = lock

	return vm, err
}

// List lists all vm's that use qemu virtualization
func (p *WSLVirtualization) List(_ machine.ListOptions) ([]*machine.ListResponse, error) {
	return GetVMInfos()
}

func GetVMInfos() ([]*machine.ListResponse, error) {
	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return nil, err
	}

	var listed []*machine.ListResponse

	if err = filepath.WalkDir(vmConfigDir, func(path string, d fs.DirEntry, err error) error {
		if strings.HasSuffix(d.Name(), ".json") {
			path := filepath.Join(vmConfigDir, d.Name())
			vm, err := readAndMigrate(path, strings.TrimSuffix(d.Name(), ".json"))
			if err != nil {
				return err
			}
			listEntry := new(machine.ListResponse)

			listEntry.Name = vm.Name
			listEntry.Stream = vm.ImageStream
			listEntry.VMType = "wsl"
			listEntry.CPUs, _ = getCPUs(vm)
			listEntry.Memory, _ = getMem(vm)
			listEntry.DiskSize = getDiskSize(vm)
			listEntry.RemoteUsername = vm.RemoteUsername
			listEntry.Port = vm.Port
			listEntry.IdentityPath = vm.IdentityPath
			listEntry.Starting = false
			listEntry.UserModeNetworking = vm.UserModeNetworking

			running := vm.isRunning()
			listEntry.CreatedAt, listEntry.LastUp, _ = vm.updateTimeStamps(running)
			listEntry.Running = running

			listed = append(listed, listEntry)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return listed, err
}

func (p *WSLVirtualization) IsValidVMName(name string) (bool, error) {
	infos, err := GetVMInfos()
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

// RemoveAndCleanMachines removes all machine and cleans up any other files associated with podman machine
func (p *WSLVirtualization) RemoveAndCleanMachines() error {
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

func (p *WSLVirtualization) VMType() define.VMType {
	return vmtype
}
