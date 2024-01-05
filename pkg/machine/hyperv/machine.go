//go:build windows

package hyperv

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/containers/common/pkg/config"
	gvproxy "github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/containers/libhvee/pkg/hypervctl"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/define"
	"github.com/containers/podman/v4/pkg/machine/hyperv/vsock"
	"github.com/containers/podman/v4/pkg/machine/ignition"
	"github.com/containers/podman/v4/pkg/machine/vmconfigs"
	"github.com/containers/podman/v4/pkg/strongunits"
	"github.com/containers/podman/v4/pkg/systemd/parser"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/podman/v4/utils"
	"github.com/containers/storage/pkg/lockfile"
	psutil "github.com/shirou/gopsutil/v3/process"
	"github.com/sirupsen/logrus"
)

var (
	// vmtype refers to qemu (vs libvirt, krun, etc).
	vmtype = define.HyperVVirt
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

const hyperVVsockNMConnection = `
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

type HyperVMachine struct {
	// ConfigPath is the fully qualified path to the configuration file
	ConfigPath define.VMFile
	// HostUser contains info about host user
	vmconfigs.HostUser
	// ImageConfig describes the bootable image
	machine.ImageConfig
	// Mounts is the list of remote filesystems to mount
	Mounts []vmconfigs.Mount
	// Name of VM
	Name string
	// NetworkVSock is for the user networking
	NetworkHVSock vsock.HVSockRegistryEntry
	// ReadySocket tells host when vm is booted
	ReadyHVSock vsock.HVSockRegistryEntry
	// ResourceConfig is physical attrs of the VM
	vmconfigs.ResourceConfig
	// SSHConfig for accessing the remote vm
	vmconfigs.SSHConfig
	// Starting tells us whether the machine is running or if we have just dialed it to start it
	Starting bool
	// Created contains the original created time instead of querying the file mod time
	Created time.Time
	// LastUp contains the last recorded uptime
	LastUp time.Time
	// GVProxy will write its PID here
	GvProxyPid define.VMFile
	// MountVsocks contains the currently-active vsocks, mapped to the
	// directory they should be mounted on.
	MountVsocks map[string]uint64
	// Used at runtime for serializing write operations
	lock *lockfile.LockFile
}

// addNetworkAndReadySocketsToRegistry adds the Network and Ready sockets to the
// Windows registry
func (m *HyperVMachine) addNetworkAndReadySocketsToRegistry() error {
	networkHVSock, err := vsock.NewHVSockRegistryEntry(m.Name, vsock.Network)
	if err != nil {
		return err
	}
	eventHVSocket, err := vsock.NewHVSockRegistryEntry(m.Name, vsock.Events)
	if err != nil {
		return err
	}
	m.NetworkHVSock = *networkHVSock
	m.ReadyHVSock = *eventHVSocket
	return nil
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
		err error
	)

	// cleanup half-baked files if init fails at any point
	callbackFuncs := machine.InitCleanup()
	defer callbackFuncs.CleanIfErr(&err)
	go callbackFuncs.CleanOnSignal()

	callbackFuncs.Add(m.ImagePath.Delete)
	callbackFuncs.Add(m.ConfigPath.Delete)
	callbackFuncs.Add(m.unregisterMachine)

	// Parsing here is confusing.
	// Basically, we have two paths: a source path, on the Windows machine,
	// with all that entails (drive letter, backslash separator, etc) and a
	// dest path, in the Linux machine, normal Unix semantics. They are
	// separated by a : character, with source path first, dest path second.
	// So we split on :, first two parts are guaranteed to be Windows (the
	// drive letter and file path), next one is Linux. Options, when we get
	// around to those, would be another : after that.
	// TODO: Need to support options here
	for _, mount := range opts.Volumes {
		newMount := vmconfigs.Mount{}

		splitMount := strings.Split(mount, ":")
		if len(splitMount) < 3 {
			return false, fmt.Errorf("volumes must be specified as source:destination and must be absolute")
		}
		newMount.Target = splitMount[2]
		newMount.Source = strings.Join(splitMount[:2], ":")
		if len(splitMount) > 3 {
			return false, fmt.Errorf("volume options are not presently supported on Hyper-V")
		}

		m.Mounts = append(m.Mounts, newMount)
	}

	if err = m.addNetworkAndReadySocketsToRegistry(); err != nil {
		return false, err
	}
	callbackFuncs.Add(func() error {
		m.removeNetworkAndReadySocketsFromRegistry()
		return nil
	})

	m.IdentityPath = util.GetIdentityPath(define.DefaultIdentityName)

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
	callbackFuncs.Add(m.removeSystemConnections)

	if len(opts.IgnitionPath) < 1 {
		key, err = machine.GetSSHKeys(m.IdentityPath)
		if err != nil {
			return false, err
		}
	}

	m.ResourceConfig = vmconfigs.ResourceConfig{
		CPUs:     opts.CPUS,
		DiskSize: opts.DiskSize,
		Memory:   opts.Memory,
	}
	m.Rootful = opts.Rootful

	builder := ignition.NewIgnitionBuilder(ignition.DynamicIgnition{
		Name:      m.RemoteUsername,
		Key:       key,
		VMName:    m.Name,
		VMType:    define.HyperVVirt,
		TimeZone:  opts.TimeZone,
		WritePath: m.IgnitionFile.GetPath(),
		UID:       m.UID,
		Rootful:   m.Rootful,
	})

	// If the user provides an ignition file, we need to
	// copy it into the conf dir
	if len(opts.IgnitionPath) > 0 {
		err = builder.BuildWithIgnitionFile(opts.IgnitionPath)
		return false, err
	}
	callbackFuncs.Add(m.IgnitionFile.Delete)

	if err := m.writeConfig(); err != nil {
		return false, err
	}

	if err := builder.GenerateIgnitionConfig(); err != nil {
		return false, err
	}

	readyOpts := ignition.ReadyUnitOpts{Port: m.ReadyHVSock.Port}
	readyUnitFile, err := ignition.CreateReadyUnitFile(define.HyperVVirt, &readyOpts)
	if err != nil {
		return false, err
	}

	builder.WithUnit(ignition.Unit{
		Enabled:  ignition.BoolToPtr(true),
		Name:     "ready.service",
		Contents: ignition.StrToPtr(readyUnitFile),
	})

	netUnitFile, err := createNetworkUnit(m.NetworkHVSock.Port)
	if err != nil {
		return false, err
	}

	builder.WithUnit(ignition.Unit{
		Contents: ignition.StrToPtr(netUnitFile),
		Enabled:  ignition.BoolToPtr(true),
		Name:     "vsock-network.service",
	})

	builder.WithFile(ignition.File{
		Node: ignition.Node{
			Path: "/etc/NetworkManager/system-connections/vsock0.nmconnection",
		},
		FileEmbedded1: ignition.FileEmbedded1{
			Append: nil,
			Contents: ignition.Resource{
				Source: ignition.EncodeDataURLPtr(hyperVVsockNMConnection),
			},
			Mode: ignition.IntToPtr(0600),
		},
	})

	if err := builder.Build(); err != nil {
		return false, err
	}

	if err = m.resizeDisk(strongunits.GiB(opts.DiskSize)); err != nil {
		return false, err
	}
	// The ignition file has been written. We now need to
	// read it so that we can put it into key-value pairs
	err = m.readAndSplitIgnition()
	return err == nil, err
}

func createNetworkUnit(netPort uint64) (string, error) {
	netUnit := parser.NewUnitFile()
	netUnit.Add("Unit", "Description", "vsock_network")
	netUnit.Add("Unit", "After", "NetworkManager.service")
	netUnit.Add("Service", "ExecStart", fmt.Sprintf("/usr/libexec/podman/gvforwarder -preexisting -iface vsock0 -url vsock://2:%d/connect", netPort))
	netUnit.Add("Service", "ExecStartPost", "/usr/bin/nmcli c up vsock0")
	netUnit.Add("Install", "WantedBy", "multi-user.target")
	return netUnit.ToString()
}

func (m *HyperVMachine) unregisterMachine() error {
	vmm := hypervctl.NewVirtualMachineManager()
	vm, err := vmm.GetMachine(m.Name)
	if err != nil {
		logrus.Error(err)
	}
	return vm.Remove("")
}

func (m *HyperVMachine) removeSystemConnections() error {
	return machine.RemoveConnections(m.Name, fmt.Sprintf("%s-root", m.Name))
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

	vmState, err := stateConversion(vm.State())
	if err != nil {
		return nil, err
	}

	podmanSocket, err := m.forwardSocketPath()
	if err != nil {
		return nil, err
	}

	return &machine.InspectInfo{
		ConfigPath: m.ConfigPath,
		ConnectionInfo: machine.ConnectionConfig{
			PodmanSocket: podmanSocket,
		},
		Created: m.Created,
		Image: machine.ImageConfig{
			IgnitionFile: m.IgnitionFile,
			ImageStream:  "",
			ImagePath:    m.ImagePath,
		},
		LastUp: m.LastUp,
		Name:   m.Name,
		Resources: vmconfigs.ResourceConfig{
			CPUs:     uint64(cfg.Hardware.CPUs),
			DiskSize: 0,
			Memory:   cfg.Hardware.Memory,
		},
		SSHConfig: m.SSHConfig,
		State:     string(vmState),
		Rootful:   m.Rootful,
	}, nil
}

// collectFilesToDestroy retrieves the files that will be destroyed by `Remove`
func (m *HyperVMachine) collectFilesToDestroy(opts machine.RemoveOptions, diskPath *string) []string {
	files := []string{}
	if !opts.SaveIgnition {
		files = append(files, m.IgnitionFile.GetPath())
	}

	if !opts.SaveImage {
		*diskPath = m.ImagePath.GetPath()
		files = append(files, *diskPath)
	}

	files = append(files, m.ConfigPath.GetPath())
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
	m.lock.Lock()
	defer m.lock.Unlock()

	vmm := hypervctl.NewVirtualMachineManager()
	vm, err := vmm.GetMachine(m.Name)
	if err != nil {
		return "", nil, fmt.Errorf("getting virtual machine: %w", err)
	}
	// In hyperv, they call running 'enabled'
	if vm.State() == hypervctl.Enabled {
		if !opts.Force {
			return "", nil, &machine.ErrVMRunningCannotDestroyed{Name: m.Name}
		}
		// force stop bc we are destroying
		if err := vm.StopWithForce(); err != nil {
			return "", nil, fmt.Errorf("stopping virtual machine: %w", err)
		}

		// Update state on the VM by pulling its info again
		vm, err = vmm.GetMachine(m.Name)
		if err != nil {
			return "", nil, fmt.Errorf("getting VM: %w", err)
		}
	}

	// Tear down vsocks
	if err := m.removeShares(); err != nil {
		logrus.Errorf("Error removing vsock: %w", err)
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
		if err := vm.Remove(""); err != nil {
			return fmt.Errorf("removing virtual machine: %w", err)
		}
		return nil
	}, nil
}

func (m *HyperVMachine) Set(name string, opts machine.SetOptions) ([]error, error) {
	var (
		cpuChanged, memoryChanged bool
		setErrors                 []error
	)

	m.lock.Lock()
	defer m.lock.Unlock()

	vmm := hypervctl.NewVirtualMachineManager()
	// Considering this a hard return if we cannot lookup the machine
	vm, err := vmm.GetMachine(m.Name)
	if err != nil {
		return setErrors, fmt.Errorf("getting machine: %w", err)
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
		newDiskSize := strongunits.GiB(*opts.DiskSize)
		if err := m.resizeDisk(newDiskSize); err != nil {
			setErrors = append(setErrors, err)
		}
	}
	if opts.CPUs != nil && m.CPUs != *opts.CPUs {
		m.CPUs = *opts.CPUs
		cpuChanged = true
	}
	if opts.Memory != nil && m.Memory != *opts.Memory {
		m.Memory = *opts.Memory
		memoryChanged = true
	}

	if opts.USBs != nil {
		setErrors = append(setErrors, errors.New("changing USBs not supported for hyperv machines"))
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
			setErrors = append(setErrors, fmt.Errorf("setting CPU and Memory for VM: %w", err))
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
	if state != define.Running {
		return fmt.Errorf("vm %q is not running", m.Name)
	}

	username := opts.Username
	if username == "" {
		username = m.RemoteUsername
	}
	return machine.CommonSSH(username, m.IdentityPath, m.Name, m.Port, opts.Args)
}

func (m *HyperVMachine) Start(name string, opts machine.StartOptions) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Start 9p shares
	shares, err := m.createShares()
	if err != nil {
		return err
	}
	m.MountVsocks = shares
	if err := m.writeConfig(); err != nil {
		return err
	}

	vmm := hypervctl.NewVirtualMachineManager()
	vm, err := vmm.GetMachine(m.Name)
	if err != nil {
		return err
	}
	if vm.State() != hypervctl.Disabled {
		return hypervctl.ErrMachineStateInvalid
	}
	gvproxyPid, _, _, err := m.startHostNetworking()
	if err != nil {
		return fmt.Errorf("unable to start host networking: %q", err)
	}

	// The "starting" status from hyper v is a very small windows and not really
	// the same as what we want.  so modeling starting behaviour after qemu
	m.Starting = true
	if err := m.writeConfig(); err != nil {
		return fmt.Errorf("writing JSON file: %w", err)
	}

	if err := vm.Start(); err != nil {
		return err
	}
	// Wait on notification from the guest
	if err := m.ReadyHVSock.Listen(); err != nil {
		return err
	}

	// set starting back false now that we are running
	m.Starting = false

	if m.HostUser.Modified {
		if machine.UpdatePodmanDockerSockService(m, name, m.UID, m.Rootful) == nil {
			// Reset modification state if there are no errors, otherwise ignore errors
			// which are already logged
			m.HostUser.Modified = false
		}
	}

	// Write the config with updated starting status and hostuser modification
	if err := m.writeConfig(); err != nil {
		return err
	}

	// Check if gvproxy is still running.
	// Do this *after* we write config, so we have still recorded that the
	// VM is actually running - to ensure that stopping the machine works as
	// expected.
	_, err = psutil.NewProcess(gvproxyPid)
	if err != nil {
		return fmt.Errorf("gvproxy appears to have stopped (PID %d): %w", gvproxyPid, err)
	}

	// Finalize starting shares after we are confident gvproxy is still alive.
	if err := m.startShares(); err != nil {
		return err
	}

	return nil
}

func (m *HyperVMachine) State(_ bool) (define.Status, error) {
	vmm := hypervctl.NewVirtualMachineManager()
	vm, err := vmm.GetMachine(m.Name)
	if err != nil {
		return "", err
	}
	if vm.IsStarting() {
		return define.Starting, nil
	}
	if vm.State() == hypervctl.Enabled {
		return define.Running, nil
	}
	// Following QEMU pattern here where only three
	// states seem valid
	return define.Stopped, nil
}

func (m *HyperVMachine) Stop(name string, opts machine.StopOptions) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	vmm := hypervctl.NewVirtualMachineManager()
	vm, err := vmm.GetMachine(m.Name)
	if err != nil {
		return fmt.Errorf("getting virtual machine: %w", err)
	}
	vmState := vm.State()
	if vm.State() == hypervctl.Disabled {
		return nil
	}
	if vmState != hypervctl.Enabled { // more states could be provided as well
		return hypervctl.ErrMachineStateInvalid
	}

	if err := machine.CleanupGVProxy(m.GvProxyPid); err != nil {
		logrus.Error(err)
	}

	if err := vm.Stop(); err != nil {
		return fmt.Errorf("stopping virtual machine: %w", err)
	}

	// keep track of last up
	m.LastUp = time.Now()
	return m.writeConfig()
}

func (m *HyperVMachine) jsonConfigPath() (string, error) {
	configDir, err := machine.GetConfDir(define.HyperVVirt)
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
		if errors.Is(err, machine.ErrNoSuchVM) {
			return nil, &machine.ErrVMDoesNotExist{Name: m.Name}
		}
		return nil, err
	}

	lock, err := machine.GetLock(mm.Name, vmtype)
	if err != nil {
		return nil, err
	}
	mm.lock = lock

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
			return machine.ErrNoSuchVM
		}
		return err
	}
	return json.Unmarshal(b, m)
}

func (m *HyperVMachine) startHostNetworking() (int32, string, machine.APIForwardingState, error) {
	var (
		forwardSock string
		state       machine.APIForwardingState
	)
	cfg, err := config.Default()
	if err != nil {
		return -1, "", machine.NoForwarding, err
	}

	executable, err := os.Executable()
	if err != nil {
		return -1, "", 0, fmt.Errorf("unable to locate executable: %w", err)
	}

	gvproxyBinary, err := cfg.FindHelperBinary("gvproxy.exe", false)
	if err != nil {
		return -1, "", 0, err
	}

	cmd := gvproxy.NewGvproxyCommand()
	cmd.SSHPort = m.Port
	cmd.AddEndpoint(fmt.Sprintf("vsock://%s", m.NetworkHVSock.KeyName))
	cmd.PidFile = m.GvProxyPid.GetPath()

	cmd, forwardSock, state = m.setupAPIForwarding(cmd)
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		cmd.Debug = true
	}

	c := cmd.Cmd(gvproxyBinary)

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		if err := logCommandToFile(c, "gvproxy.log"); err != nil {
			return -1, "", 0, err
		}
	}

	logrus.Debugf("Starting gvproxy with command: %s %v", gvproxyBinary, c.Args)

	if err := c.Start(); err != nil {
		return -1, "", 0, fmt.Errorf("unable to execute: %s: %w", cmd.ToCmdline(), err)
	}

	logrus.Debugf("Got gvproxy PID as %d", c.Process.Pid)

	if len(m.MountVsocks) == 0 {
		return int32(c.Process.Pid), forwardSock, state, nil
	}

	// Start the 9p server in the background
	args := []string{}
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		args = append(args, "--log-level=debug")
	}
	args = append(args, "machine", "server9p")
	for dir, vsock := range m.MountVsocks {
		for _, mount := range m.Mounts {
			if mount.Target == dir {
				args = append(args, "--serve", fmt.Sprintf("%s:%s", mount.Source, winio.VsockServiceID(uint32(vsock)).String()))
				break
			}
		}
	}
	args = append(args, fmt.Sprintf("%d", c.Process.Pid))

	logrus.Debugf("Going to start 9p server using command: %s %v", executable, args)

	fsCmd := exec.Command(executable, args...)

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		if err := logCommandToFile(fsCmd, "podman-machine-server9.log"); err != nil {
			return -1, "", 0, err
		}
	}

	if err := fsCmd.Start(); err != nil {
		return -1, "", 0, fmt.Errorf("unable to execute: %s %v: %w", executable, args, err)
	}

	logrus.Infof("Started podman 9p server as PID %d", fsCmd.Process.Pid)

	return int32(c.Process.Pid), forwardSock, state, nil
}

func logCommandToFile(c *exec.Cmd, filename string) error {
	dir, err := machine.GetDataDir(define.HyperVVirt)
	if err != nil {
		return fmt.Errorf("obtain machine dir: %w", err)
	}
	path := filepath.Join(dir, filename)
	logrus.Infof("Going to log to %s", path)
	log, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}
	defer log.Close()

	c.Stdout = log
	c.Stderr = log

	return nil
}

func (m *HyperVMachine) setupAPIForwarding(cmd gvproxy.GvproxyCommand) (gvproxy.GvproxyCommand, string, machine.APIForwardingState) {
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

	cmd.AddForwardSock(socket.GetPath())
	cmd.AddForwardDest(destSock)
	cmd.AddForwardUser(forwardUser)
	cmd.AddForwardIdentity(m.IdentityPath)

	return cmd, "", machine.MachineLocal
}

func (m *HyperVMachine) dockerSock() (string, error) {
	dd, err := machine.GetDataDir(define.HyperVVirt)
	if err != nil {
		return "", err
	}
	return filepath.Join(dd, "podman.sock"), nil
}

func (m *HyperVMachine) forwardSocketPath() (*define.VMFile, error) {
	sockName := "podman.sock"
	path, err := machine.GetDataDir(define.HyperVVirt)
	if err != nil {
		return nil, fmt.Errorf("Resolving data dir: %s", err.Error())
	}
	return define.NewMachineFile(filepath.Join(path, sockName), &sockName)
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

func (m *HyperVMachine) resizeDisk(newSize strongunits.GiB) error {
	if m.DiskSize > uint64(newSize) {
		return &machine.ErrNewDiskSizeTooSmall{OldSize: strongunits.ToGiB(strongunits.B(m.DiskSize)), NewSize: newSize}
	}
	resize := exec.Command("powershell", []string{"-command", fmt.Sprintf("Resize-VHD %s %d", m.ImagePath.GetPath(), newSize.ToBytes())}...)
	resize.Stdout = os.Stdout
	resize.Stderr = os.Stderr
	if err := resize.Run(); err != nil {
		return fmt.Errorf("resizing image: %q", err)
	}
	return nil
}

func (m *HyperVMachine) isStarting() bool {
	return m.Starting
}

func (m *HyperVMachine) createShares() (_ map[string]uint64, defErr error) {
	toReturn := make(map[string]uint64)

	for _, mount := range m.Mounts {
		var hvSock *vsock.HVSockRegistryEntry

		vsockNum, ok := m.MountVsocks[mount.Target]
		if ok {
			// Ignore errors here, we'll just try and recreate the
			// vsock below.
			testVsock, err := vsock.LoadHVSockRegistryEntry(vsockNum)
			if err == nil {
				hvSock = testVsock
			}
		}
		if hvSock == nil {
			testVsock, err := vsock.NewHVSockRegistryEntry(m.Name, vsock.Fileserver)
			if err != nil {
				return nil, err
			}
			defer func() {
				if defErr != nil {
					if err := testVsock.Remove(); err != nil {
						logrus.Errorf("Removing vsock: %v", err)
					}
				}
			}()
			hvSock = testVsock
		}

		logrus.Debugf("Going to share directory %s via 9p on vsock %d", mount.Source, hvSock.Port)

		toReturn[mount.Target] = hvSock.Port
	}

	return toReturn, nil
}

func (m *HyperVMachine) removeShares() error {
	var removalErr error

	for _, mount := range m.Mounts {
		vsockNum, ok := m.MountVsocks[mount.Target]
		if !ok {
			// Mount doesn't have a valid vsock, no need to tear down
			continue
		}

		vsock, err := vsock.LoadHVSockRegistryEntry(vsockNum)
		if err != nil {
			logrus.Debugf("Vsock %d for mountpoint %s does not have a valid registry entry, skipping removal", vsockNum, mount.Target)
			continue
		}

		if err := vsock.Remove(); err != nil {
			if removalErr != nil {
				logrus.Errorf("Error removing vsock: %w", removalErr)
			}
			removalErr = fmt.Errorf("removing vsock %d for mountpoint %s: %w", vsockNum, mount.Target, err)
		}
	}

	return removalErr
}

func (m *HyperVMachine) startShares() error {
	for mountpoint, sockNum := range m.MountVsocks {
		args := []string{"-q", "--", "sudo", "podman"}
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			args = append(args, "--log-level=debug")
		}
		args = append(args, "machine", "client9p", fmt.Sprintf("%d", sockNum), mountpoint)

		sshOpts := machine.SSHOptions{
			Args: args,
		}

		if err := m.SSH(m.Name, sshOpts); err != nil {
			return err
		}
	}

	return nil
}
