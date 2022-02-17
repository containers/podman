//go:build (amd64 && !windows) || !arm64
// +build amd64,!windows !arm64

package vbox

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/utils"
	"github.com/docker/go-units"
	"github.com/pkg/errors"
)

var (
	vboxProvider  = &Provider{}
	vmtype        = "vbox"
	VBoxManageCmd = "VBoxManage"
	VBOSType      = "Fedora_64"
)

const (
	DefaultMachineName = "podman-machine-vbox"
)

func GetVBoxProvider() machine.Provider {
	return vboxProvider
}

func (vbp *Provider) DefaultVMName() string {
	return DefaultMachineName
}

func (vbp *Provider) NewMachine(opts machine.InitOptions) (machine.VM, error) {
	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return nil, err
	}
	vbm := new(MachineVM)
	if len(opts.Name) > 0 {
		vbm.Name = opts.Name
	}
	ignitionFile := filepath.Join(vmConfigDir, vbm.Name+".ign")
	vbm.IgnitionFilePath = ignitionFile

	vbm.ImagePath = opts.ImagePath
	vbm.RemoteUsername = opts.Username

	// Add a random port for ssh
	port, err := utils.GetRandomPort()
	if err != nil {
		return nil, err
	}
	vbm.Port = port

	vbm.CPUs = opts.CPUS
	vbm.Memory = opts.Memory
	if opts.DiskSize < 8 {
		return vbm, fmt.Errorf("disk size cannot be less than 8GB, specified: %d", opts.DiskSize)
	}
	vbm.DiskSize = opts.DiskSize

	// Look up the executable
	execPath, err := exec.LookPath(VBoxManageCmd)
	if err != nil {
		return nil, err
	}
	vbm.VBoxManageExecPath = execPath
	vbm.VBOSType = VBOSType
	return vbm, nil
}

func (vbp *Provider) LoadVMByName(name string) (machine.VM, error) {
	vbm := new(MachineVM)
	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return nil, err
	}
	b, err := ioutil.ReadFile(filepath.Join(vmConfigDir, name+".json"))
	if os.IsNotExist(err) {
		return nil, errors.Wrap(machine.ErrNoSuchVM, name)
	}
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(b, vbm)

	return vbm, err
}

func getList() ([]*machine.ListResponse, error) {
	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return nil, err
	}

	var listed []*machine.ListResponse

	if err = filepath.Walk(vmConfigDir, func(path string, info os.FileInfo, err error) error {
		vbm := new(MachineVM)
		if strings.HasSuffix(info.Name(), ".json") {
			fullPath := filepath.Join(vmConfigDir, info.Name())
			b, err := ioutil.ReadFile(fullPath)
			if err != nil {
				return err
			}
			err = json.Unmarshal(b, vbm)
			if err != nil {
				return err
			}
			listEntry := new(machine.ListResponse)

			listEntry.Name = vbm.Name
			listEntry.Stream = vbm.ImageStream
			listEntry.VMType = vmtype
			listEntry.CPUs = vbm.CPUs
			listEntry.Memory = vbm.Memory * units.MiB
			listEntry.DiskSize = vbm.DiskSize * units.GiB
			listEntry.Port = vbm.Port
			listEntry.RemoteUsername = vbm.RemoteUsername
			listEntry.IdentityPath = vbm.IdentityPath
			fi, err := os.Stat(fullPath)
			if err != nil {
				return err
			}
			listEntry.CreatedAt = fi.ModTime()

			fi, err = os.Stat(vbm.VIDPath)
			if err != nil {
				return err
			}
			listEntry.LastUp = fi.ModTime()
			running, err := vbm.isRunning()
			if err != nil {
				return err
			}
			if running {
				listEntry.Running = true
			}

			listed = append(listed, listEntry)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return listed, err
}

func (vbp *Provider) List(_ machine.ListOptions) ([]*machine.ListResponse, error) {
	return getList()
}

func (vbp *Provider) IsValidVMName(name string) (bool, error) {
	infos, err := getList()
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

func (vbp *Provider) CheckExclusiveActiveVM() (bool, string, error) {
	// Dosen't matter for VBox
	return false, "", nil
}
