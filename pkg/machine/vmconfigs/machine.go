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

	"github.com/containers/podman/v4/pkg/machine/connection"

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
func NewMachineConfig(opts define.InitOptions, dirs *define.MachineDirs, sshIdentityPath string) (*MachineConfig, error) {
	mc := new(MachineConfig)
	mc.Name = opts.Name
	mc.dirs = dirs

	machineLock, err := lock.GetMachineLock(opts.Name, dirs.ConfigDir.GetPath())
	if err != nil {
		return nil, err
	}
	mc.lock = machineLock

	// Assign Dirs
	cf, err := define.NewMachineFile(filepath.Join(dirs.ConfigDir.GetPath(), fmt.Sprintf("%s.json", opts.Name)), nil)
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

	sshConfig := SSHConfig{
		IdentityPath:   sshIdentityPath,
		Port:           sshPort,
		RemoteUsername: opts.Username,
	}

	mc.SSH = sshConfig
	mc.Created = time.Now()

	mc.HostUser = HostUser{UID: getHostUID(), Rootful: opts.Rootful}

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

// Refresh reloads the config file from disk
func (mc *MachineConfig) Refresh() error {
	content, err := os.ReadFile(mc.configPath.GetPath())
	if err != nil {
		return err
	}
	return json.Unmarshal(content, mc)
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

func (mc *MachineConfig) Remove(saveIgnition, saveImage bool) ([]string, func() error, error) {
	ignitionFile, err := mc.IgnitionFile()
	if err != nil {
		return nil, nil, err
	}

	readySocket, err := mc.ReadySocket()
	if err != nil {
		return nil, nil, err
	}

	logPath, err := mc.LogFile()
	if err != nil {
		return nil, nil, err
	}

	rmFiles := []string{
		mc.configPath.GetPath(),
		readySocket.GetPath(),
		logPath.GetPath(),
	}
	if !saveImage {
		mc.ImagePath.GetPath()
	}
	if !saveIgnition {
		ignitionFile.GetPath()
	}

	mcRemove := func() error {
		if !saveIgnition {
			if err := ignitionFile.Delete(); err != nil {
				logrus.Error(err)
			}
		}
		if !saveImage {
			if err := mc.ImagePath.Delete(); err != nil {
				logrus.Error(err)
			}
		}
		if err := mc.configPath.Delete(); err != nil {
			logrus.Error(err)
		}
		if err := readySocket.Delete(); err != nil {
			logrus.Error()
		}
		if err := logPath.Delete(); err != nil {
			logrus.Error(err)
		}
		// TODO This should be bumped up into delete and called out in the text given then
		// are not technically files per'se
		return connection.RemoveConnections(mc.Name, mc.Name+"-root")
	}

	return rmFiles, mcRemove, nil
}

// ConfigDir is a simple helper to obtain the machine config dir
func (mc *MachineConfig) ConfigDir() (*define.VMFile, error) {
	if mc.dirs == nil || mc.dirs.ConfigDir == nil {
		return nil, errors.New("no configuration directory set")
	}
	return mc.dirs.ConfigDir, nil
}

// DataDir is a simple helper function to obtain the machine data dir
func (mc *MachineConfig) DataDir() (*define.VMFile, error) {
	if mc.dirs == nil || mc.dirs.DataDir == nil {
		return nil, errors.New("no data directory set")
	}
	return mc.dirs.DataDir, nil
}

// RuntimeDir is simple helper function to obtain the runtime dir
func (mc *MachineConfig) RuntimeDir() (*define.VMFile, error) {
	if mc.dirs == nil || mc.dirs.RuntimeDir == nil {
		return nil, errors.New("no runtime directory set")
	}
	return mc.dirs.RuntimeDir, nil
}

func (mc *MachineConfig) SetDirs(dirs *define.MachineDirs) {
	mc.dirs = dirs
}

func (mc *MachineConfig) IgnitionFile() (*define.VMFile, error) {
	configDir, err := mc.ConfigDir()
	if err != nil {
		return nil, err
	}
	return configDir.AppendToNewVMFile(mc.Name+".ign", nil)
}

func (mc *MachineConfig) ReadySocket() (*define.VMFile, error) {
	rtDir, err := mc.RuntimeDir()
	if err != nil {
		return nil, err
	}
	return rtDir.AppendToNewVMFile(mc.Name+".sock", nil)
}

func (mc *MachineConfig) LogFile() (*define.VMFile, error) {
	rtDir, err := mc.RuntimeDir()
	if err != nil {
		return nil, err
	}
	return rtDir.AppendToNewVMFile(mc.Name+".log", nil)
}

func (mc *MachineConfig) Kind() (define.VMType, error) {
	// Not super in love with this approach
	if mc.QEMUHypervisor != nil {
		return define.QemuVirt, nil
	}
	if mc.AppleHypervisor != nil {
		return define.AppleHvVirt, nil
	}
	if mc.HyperVHypervisor != nil {
		return define.HyperVVirt, nil
	}
	if mc.WSLHypervisor != nil {
		return define.WSLVirt, nil
	}

	return define.UnknownVirt, nil
}

// LoadMachineByName returns a machine config based on the vm name and provider
func LoadMachineByName(name string, dirs *define.MachineDirs) (*MachineConfig, error) {
	fullPath, err := dirs.ConfigDir.AppendToNewVMFile(name+".json", nil)
	if err != nil {
		return nil, err
	}
	mc, err := loadMachineFromFQPath(fullPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, &define.ErrVMDoesNotExist{Name: name}
		}
		return nil, err
	}
	mc.dirs = dirs
	mc.configPath = fullPath
	return mc, nil
}

// loadMachineFromFQPath stub function for loading a JSON configuration file and returning
// a machineconfig.  this should only be called if you know what you are doing.
func loadMachineFromFQPath(path *define.VMFile) (*MachineConfig, error) {
	mc := new(MachineConfig)
	b, err := path.Read()
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(b, mc); err != nil {
		return nil, fmt.Errorf("unable to load machine config file: %q", err)
	}
	lock, err := lock.GetMachineLock(mc.Name, filepath.Dir(path.GetPath()))
	mc.lock = lock
	return mc, err
}

// LoadMachinesInDir returns all the machineconfigs located in given dir
func LoadMachinesInDir(dirs *define.MachineDirs) (map[string]*MachineConfig, error) {
	mcs := make(map[string]*MachineConfig)
	if err := filepath.WalkDir(dirs.ConfigDir.GetPath(), func(path string, d fs.DirEntry, err error) error {
		if strings.HasSuffix(d.Name(), ".json") {
			fullPath, err := dirs.ConfigDir.AppendToNewVMFile(d.Name(), nil)
			if err != nil {
				return err
			}
			mc, err := loadMachineFromFQPath(fullPath)
			if err != nil {
				return err
			}
			mc.configPath = fullPath
			mc.dirs = dirs
			mcs[mc.Name] = mc
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return mcs, nil
}
