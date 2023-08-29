//go:build windows
// +build windows

package hyperv

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/containers/common/pkg/config"
	"github.com/containers/libhvee/pkg/hypervctl"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/podman/v4/utils"
	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
)

var (
	// vmtype refers to qemu (vs libvirt, krun, etc).
	vmtype = machine.HyperVVirt
)

const (
	// Some of this will need to change when we are closer to having
	// working code.
	VolumeTypeVirtfs     = "virtfs"
	MountType9p          = "9p"
	dockerSockPath       = "/var/run/docker.sock"
	dockerConnectTimeout = 5 * time.Second
	apiUpTimeout         = 20 * time.Second
)

type HyperVMachine struct {
	// ConfigPath is the fully qualified path to the configuration file
	ConfigPath machine.VMFile
	// HostUser contains info about host user
	machine.HostUser
	// ImageConfig describes the bootable image
	machine.ImageConfig
	// Mounts is the list of remote filesystems to mount
	Mounts []machine.Mount
	// Name of VM
	Name string
	// NetworkVSock is for the user networking
	NetworkHVSock HVSockRegistryEntry
	// ReadySocket tells host when vm is booted
	ReadyHVSock HVSockRegistryEntry
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
	// GVProxy will write its PID here
	GvProxyPid machine.VMFile
}

// addNetworkAndReadySocketsToRegistry adds the Network and Ready sockets to the
// Windows registry
func (m *HyperVMachine) addNetworkAndReadySocketsToRegistry() error {
	networkHVSock, err := NewHVSockRegistryEntry(m.Name, Network)
	if err != nil {
		return err
	}
	eventHVSocket, err := NewHVSockRegistryEntry(m.Name, Events)
	if err != nil {
		return err
	}
	m.NetworkHVSock = *networkHVSock
	m.ReadyHVSock = *eventHVSocket
	return nil
}

// writeIgnitionConfigFile generates the ignition config and writes it to the
// filesystem
func (m *HyperVMachine) writeIgnitionConfigFile(opts machine.InitOptions, user, key string) error {
	ign := machine.DynamicIgnition{
		Name:      user,
		Key:       key,
		VMName:    m.Name,
		VMType:    machine.HyperVVirt,
		TimeZone:  opts.TimeZone,
		WritePath: m.IgnitionFile.GetPath(),
		UID:       m.UID,
	}

	if err := ign.GenerateIgnitionConfig(); err != nil {
		return err
	}

	// ready is a unit file that sets up the virtual serial device
	// where when the VM is done configuring, it will send an ack
	// so a listening host knows it can being interacting with it
	//
	// VSOCK-CONNECT:2 <- shortcut to connect to the hostvm
	ready := `[Unit]
After=remove-moby.service sshd.socket sshd.service
After=systemd-user-sessions.service
OnFailure=emergency.target
OnFailureJobMode=isolate
[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/bin/sh -c '/usr/bin/echo Ready | socat - VSOCK-CONNECT:2:%d'
[Install]
RequiredBy=default.target
`
	readyUnit := machine.Unit{
		Enabled:  machine.BoolToPtr(true),
		Name:     "ready.service",
		Contents: machine.StrToPtr(fmt.Sprintf(ready, m.ReadyHVSock.Port)),
	}

	// userNetwork is a systemd unit file that calls the vm helpoer utility
	// needed to take traffic from a network vsock0 device to the actual vsock
	// and onto the host
	userNetwork := `
[Unit]
Description=vsock_network
After=NetworkManager.service

[Service]
ExecStart=/usr/libexec/podman/vm -preexisting -iface vsock0 -url vsock://2:%d/connect
ExecStartPost=/usr/bin/nmcli c up vsock0

[Install]
WantedBy=multi-user.target
`
	vsockNetUnit := machine.Unit{
		Contents: machine.StrToPtr(fmt.Sprintf(userNetwork, m.NetworkHVSock.Port)),
		Enabled:  machine.BoolToPtr(true),
		Name:     "vsock-network.service",
	}

	ign.Cfg.Systemd.Units = append(ign.Cfg.Systemd.Units, readyUnit, vsockNetUnit)

	vSockNMConnection := `
[connection]
id=vsock0
type=tun
interface-name=vsock0

[tun]
mode=2

[802-3-ethernet]
cloned-mac-address=5A:94:EF:E4:0C:EE

[ipv4]
method=auto

[proxy]
`

	ign.Cfg.Storage.Files = append(ign.Cfg.Storage.Files, machine.File{
		Node: machine.Node{
			Path: "/etc/NetworkManager/system-connections/vsock0.nmconnection",
		},
		FileEmbedded1: machine.FileEmbedded1{
			Append: nil,
			Contents: machine.Resource{
				Source: machine.EncodeDataURLPtr(vSockNMConnection),
			},
			Mode: machine.IntToPtr(0600),
		},
	})

	return ign.Write()
}

// readAndSplitIgnition reads the ignition file and splits it into key:value pairs
func (m *HyperVMachine) readAndSplitIgnition() error {
	ignFile, err := m.IgnitionFile.Read()
	if err != nil {
		return err
	}
	reader := bytes.NewReader(ignFile)

	vm, err := hypervctl.NewVirtualMachineManager().GetMachine(m.Name)
	if err != nil {
		return err
	}
	return vm.SplitAndAddIgnition("ignition.config.", reader)
}

func (m *HyperVMachine) Init(opts machine.InitOptions) (bool, error) {
	var (
		key string
	)

	if err := m.addNetworkAndReadySocketsToRegistry(); err != nil {
		return false, err
	}

	m.IdentityPath = util.GetIdentityPath(m.Name)

	if m.UID == 0 {
		m.UID = 1000
	}

	sshPort, err := utils.GetRandomPort()
	if err != nil {
		return false, err
	}
	m.Port = sshPort

	m.RemoteUsername = opts.Username
	err = machine.AddSSHConnectionsToPodmanSocket(
		m.UID,
		m.Port,
		m.IdentityPath,
		m.Name,
		m.RemoteUsername,
		opts,
	)
	if err != nil {
		return false, err
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
	if err := m.writeConfig(); err != nil {
		return false, err
	}

	// Write the ignition file
	if err := m.writeIgnitionConfigFile(opts, m.RemoteUsername, key); err != nil {
		return false, err
	}
	// The ignition file has been written. We now need to
	// read it so that we can put it into key-value pairs
	err = m.readAndSplitIgnition()
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
		Rootful:   m.Rootful,
	}, nil
}

// collectFilesToDestroy retrieves the files that will be destroyed by `Remove`
func (m *HyperVMachine) collectFilesToDestroy(opts machine.RemoveOptions, diskPath *string) []string {
	files := []string{}

	if !opts.SaveKeys {
		files = append(files, m.IdentityPath, m.IdentityPath+".pub")
	}
	if !opts.SaveIgnition {
		files = append(files, m.IgnitionFile.GetPath())
	}

	if !opts.SaveImage {
		*diskPath = m.ImagePath.GetPath()
		files = append(files, *diskPath)
	}

	files = append(files, getVMConfigPath(m.ConfigPath.GetPath(), m.Name))
	return files
}

// removeNetworkAndReadySocketsFromRegistry removes the Network and Ready sockets
// from the Windows Registry
func (m *HyperVMachine) removeNetworkAndReadySocketsFromRegistry() {
	// Remove the HVSOCK for networking
	if err := m.NetworkHVSock.Remove(); err != nil {
		logrus.Errorf("unable to remove registry entry for %s: %q", m.NetworkHVSock.KeyName, err)
	}

	// Remove the HVSOCK for events
	if err := m.ReadyHVSock.Remove(); err != nil {
		logrus.Errorf("unable to remove registry entry for %s: %q", m.ReadyHVSock.KeyName, err)
	}
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
	files = m.collectFilesToDestroy(opts, &diskPath)

	confirmationMessage := "\nThe following files will be deleted:\n\n"
	for _, msg := range files {
		confirmationMessage += msg + "\n"
	}

	confirmationMessage += "\n"
	return confirmationMessage, func() error {
		machine.RemoveFilesAndConnections(files, m.Name, m.Name+"-root")
		m.removeNetworkAndReadySocketsFromRegistry()
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
		if err := m.setRootful(*opts.Rootful); err != nil {
			setErrors = append(setErrors, fmt.Errorf("failed to set rootful option: %w", err))
		} else {
			m.Rootful = *opts.Rootful
		}
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

	if cpuChanged || memoryChanged {
		err := vm.UpdateProcessorMemSettings(func(ps *hypervctl.ProcessorSettings) {
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
		if err != nil {
			setErrors = append(setErrors, err)
		}
	}

	if len(setErrors) > 0 {
		return setErrors, setErrors[0]
	}

	// Write the new JSON out
	// considering this a hard return if we cannot write the JSON file.
	return setErrors, m.writeConfig()
}

func (m *HyperVMachine) SSH(name string, opts machine.SSHOptions) error {
	state, err := m.State(false)
	if err != nil {
		return err
	}
	if state != machine.Running {
		return fmt.Errorf("vm %q is not running", m.Name)
	}

	username := opts.Username
	if username == "" {
		username = m.RemoteUsername
	}
	return machine.CommonSSH(username, m.IdentityPath, m.Name, m.Port, opts.Args)
}

func (m *HyperVMachine) Start(name string, opts machine.StartOptions) error {
	vmm := hypervctl.NewVirtualMachineManager()
	vm, err := vmm.GetMachine(m.Name)
	if err != nil {
		return err
	}
	if vm.State() != hypervctl.Disabled {
		return hypervctl.ErrMachineStateInvalid
	}
	_, _, err = m.startHostNetworking()
	if err != nil {
		return fmt.Errorf("unable to start host networking: %q", err)
	}
	if err := vm.Start(); err != nil {
		return err
	}
	// Wait on notification from the guest
	if err := m.ReadyHVSock.Listen(); err != nil {
		return err
	}

	if m.HostUser.Modified {
		if machine.UpdatePodmanDockerSockService(m, name, m.UID, m.Rootful) == nil {
			// Reset modification state if there are no errors, otherwise ignore errors
			// which are already logged
			m.HostUser.Modified = false
			_ = m.writeConfig()
		}
	}

	return nil
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

	if err := machine.CleanupGVProxy(m.GvProxyPid); err != nil {
		logrus.Error(err)
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

	if err := mm.loadHyperVMachineFromJSON(jsonPath); err != nil {
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

func (m *HyperVMachine) loadHyperVMachineFromJSON(fqConfigPath string) error {
	b, err := os.ReadFile(fqConfigPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%q: %w", fqConfigPath, machine.ErrNoSuchVM)
		}
		return err
	}
	return json.Unmarshal(b, m)
}

func (m *HyperVMachine) startHostNetworking() (string, machine.APIForwardingState, error) {
	var (
		forwardSock string
		state       machine.APIForwardingState
	)
	cfg, err := config.Default()
	if err != nil {
		return "", machine.NoForwarding, err
	}

	attr := new(os.ProcAttr)
	dnr, dnw, err := machine.GetDevNullFiles()
	if err != nil {
		return "", machine.NoForwarding, err
	}

	defer func() {
		if err := dnr.Close(); err != nil {
			logrus.Error(err)
		}
	}()
	defer func() {
		if err := dnw.Close(); err != nil {
			logrus.Error(err)
		}
	}()

	gvproxy, err := cfg.FindHelperBinary("gvproxy.exe", false)
	if err != nil {
		return "", 0, err
	}

	attr.Files = []*os.File{dnr, dnw, dnw}
	cmd := []string{gvproxy}
	// Add the ssh port
	cmd = append(cmd, []string{"-ssh-port", fmt.Sprintf("%d", m.Port)}...)
	cmd = append(cmd, []string{"-listen", fmt.Sprintf("vsock://%s", m.NetworkHVSock.KeyName)}...)
	cmd = append(cmd, "-pid-file", m.GvProxyPid.GetPath())

	cmd, forwardSock, state = m.setupAPIForwarding(cmd)
	if logrus.GetLevel() == logrus.DebugLevel {
		cmd = append(cmd, "--debug")
		fmt.Println(cmd)
	}
	_, err = os.StartProcess(cmd[0], cmd, attr)
	if err != nil {
		return "", 0, fmt.Errorf("unable to execute: %q: %w", cmd, err)
	}
	return forwardSock, state, nil
}

func (m *HyperVMachine) setupAPIForwarding(cmd []string) ([]string, string, machine.APIForwardingState) {
	socket, err := m.forwardSocketPath()
	if err != nil {
		return cmd, "", machine.NoForwarding
	}

	destSock := fmt.Sprintf("/run/user/%d/podman/podman.sock", m.UID)
	forwardUser := "core"

	if m.Rootful {
		destSock = "/run/podman/podman.sock"
		forwardUser = "root"
	}

	cmd = append(cmd, []string{"-forward-sock", socket.GetPath()}...)
	cmd = append(cmd, []string{"-forward-dest", destSock}...)
	cmd = append(cmd, []string{"-forward-user", forwardUser}...)
	cmd = append(cmd, []string{"-forward-identity", m.IdentityPath}...)

	return cmd, "", machine.MachineLocal
}

func (m *HyperVMachine) dockerSock() (string, error) {
	dd, err := machine.GetDataDir(machine.HyperVVirt)
	if err != nil {
		return "", err
	}
	return filepath.Join(dd, "podman.sock"), nil
}

func (m *HyperVMachine) forwardSocketPath() (*machine.VMFile, error) {
	sockName := "podman.sock"
	path, err := machine.GetDataDir(machine.HyperVVirt)
	if err != nil {
		return nil, fmt.Errorf("Resolving data dir: %s", err.Error())
	}
	return machine.NewMachineFile(filepath.Join(path, sockName), &sockName)
}

func (m *HyperVMachine) writeConfig() error {
	// Write the JSON file
	return machine.WriteConfig(m.ConfigPath.Path, m)
}

func (m *HyperVMachine) setRootful(rootful bool) error {
	if err := machine.SetRootful(rootful, m.Name, m.Name+"-root"); err != nil {
		return err
	}

	m.HostUser.Modified = true
	return nil
}
