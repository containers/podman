package vmconfigs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	define2 "github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/machine/define"
	"github.com/containers/podman/v4/pkg/machine/lock"
	"github.com/containers/podman/v4/utils"
)

/*
  info        Display machine host info					common
  init        Initialize a virtual machine				specific
  inspect     Inspect an existing machine				specific
  list        List machines								specific
  os          Manage a Podman virtual machine's OS		common
  rm          Remove an existing machine				specific
  set         Set a virtual machine setting				specific
  ssh         SSH into an existing machine				common
  start       Start an existing machine					specific
  stop        Stop an existing machine					specific
*/

var (
	SSHRemoteConnection     RemoteConnectionType = "ssh"
	DefaultIgnitionUserName                      = "core"
	ForwarderBinaryName                          = "gvproxy"
)

type RemoteConnectionType string

// NewMachineConfig creates the initial machine configuration file from cli options
func NewMachineConfig(opts define.InitOptions, machineConfigDir string) (*MachineConfig, error) {
	mc := new(MachineConfig)
	mc.Name = opts.Name

	machineLock, err := lock.GetMachineLock(opts.Name, machineConfigDir)
	if err != nil {
		return nil, err
	}
	mc.lock = machineLock

	cf, err := define.NewMachineFile(filepath.Join(machineConfigDir, fmt.Sprintf("%s.json", opts.Name)), nil)
	if err != nil {
		return nil, err
	}
	mc.configPath = cf

	// System Resources
	mrc := ResourceConfig{
		CPUs:     opts.CPUS,
		DiskSize: opts.DiskSize,
		Memory:   opts.Memory,
		USBs:     nil, // Needs to be filled in by providers?
	}
	mc.Resources = mrc

	sshPort, err := utils.GetRandomPort()
	if err != nil {
		return nil, err
	}

	// Single key examination should occur here
	sshConfig := SSHConfig{
		IdentityPath:   "/home/baude/.local/share/containers/podman/machine", // TODO Fix this
		Port:           sshPort,
		RemoteUsername: opts.Username,
	}

	mc.SSH = sshConfig
	mc.Created = time.Now()

	mc.HostUser = HostUser{UID: getHostUID(), Rootful: opts.Rootful}

	// TODO - Temporarily disabled to make things easier
	/*
		// TODO AddSSHConnectionToPodmanSocket could put converted become a method of MachineConfig
		if err := connection.AddSSHConnectionsToPodmanSocket(mc.HostUser.UID, mc.SSH.Port, mc.SSH.IdentityPath, mc.Name, mc.SSH.RemoteUsername, opts); err != nil {
			return nil, err
		}
	*/
	// addcallback for ssh connections here

	return mc, nil
}

// Lock creates a lock on the machine for single access
func (mc *MachineConfig) Lock() {
	mc.lock.Lock()
}

// Unlock removes an existing lock
func (mc *MachineConfig) Unlock() {
	mc.lock.Unlock()
}

// Write is a locking way to the machine configuration file
func (mc *MachineConfig) Write() error {
	mc.Lock()
	defer mc.Unlock()
	return mc.write()
}

// write is a non-locking way to write the machine configuration file to disk
func (mc *MachineConfig) write() error {
	if mc.configPath == nil {
		return fmt.Errorf("no configuration file associated with vm %q", mc.Name)
	}
	b, err := json.Marshal(mc)
	if err != nil {
		return err
	}
	logrus.Debugf("writing configuration file %q", mc.configPath.Path)
	return os.WriteFile(mc.configPath.GetPath(), b, define.DefaultFilePerm)
}

func (mc *MachineConfig) removeSystemConnection() error { //nolint:unused
	return define2.ErrNotImplemented
}

// updateLastBoot writes the current time to the machine configuration file. it is
// an non-locking method and assumes it is being called locked
func (mc *MachineConfig) updateLastBoot() error { //nolint:unused
	mc.LastUp = time.Now()
	return mc.Write()
}

func (mc *MachineConfig) removeMachineFiles() error { //nolint:unused
	return define2.ErrNotImplemented
}

func (mc *MachineConfig) Info() error { // signature TBD
	return define2.ErrNotImplemented
}

func (mc *MachineConfig) OSApply() error { // signature TBD
	return define2.ErrNotImplemented
}

func (mc *MachineConfig) SecureShell() error { // Used SecureShell instead of SSH to do struct collision
	return define2.ErrNotImplemented
}

func (mc *MachineConfig) Inspect() error { // signature TBD
	return define2.ErrNotImplemented
}

func (mc *MachineConfig) ConfigDir() (string, error) {
	if mc.configPath == nil {
		return "", errors.New("no configuration directory set")
	}
	return filepath.Dir(mc.configPath.GetPath()), nil
}

// LoadMachineByName returns a machine config based on the vm name and provider
func LoadMachineByName(name, configDir string) (*MachineConfig, error) {
	fullPath := filepath.Join(configDir, fmt.Sprintf("%s.json", name))
	return loadMachineFromFQPath(fullPath)
}

// loadMachineFromFQPath stub function for loading a JSON configuration file and returning
// a machineconfig.  this should only be called if you know what you are doing.
func loadMachineFromFQPath(path string) (*MachineConfig, error) {
	mc := new(MachineConfig)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(b, mc)
	return mc, err
}

// LoadMachinesInDir returns all the machineconfigs located in given dir
func LoadMachinesInDir(configDir string) (map[string]*MachineConfig, error) {
	mcs := make(map[string]*MachineConfig)
	if err := filepath.WalkDir(configDir, func(path string, d fs.DirEntry, err error) error {
		if strings.HasSuffix(d.Name(), ".json") {
			fullPath := filepath.Join(configDir, d.Name())
			mc, err := loadMachineFromFQPath(fullPath)
			if err != nil {
				return err
			}
			mcs[mc.Name] = mc
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return mcs, nil
}
