//go:build darwin

package applehv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containers/common/pkg/config"
	gvproxy "github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/applehv/vfkit"
	"github.com/containers/podman/v4/pkg/machine/define"
	"github.com/containers/podman/v4/pkg/machine/sockets"
	"github.com/containers/podman/v4/pkg/machine/vmconfigs"
    "github.com/containers/podman/v4/pkg/machine/ignition"
	"github.com/containers/podman/v4/pkg/strongunits"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/podman/v4/utils"
	"github.com/containers/storage/pkg/lockfile"
	vfConfig "github.com/crc-org/vfkit/pkg/config"
	vfRest "github.com/crc-org/vfkit/pkg/rest"
	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
)

var (
	// vmtype refers to qemu (vs libvirt, krun, etc).
	vmtype = define.AppleHvVirt
)

const (
	dockerSock           = "/var/run/docker.sock"
	dockerConnectTimeout = 5 * time.Second
	apiUpTimeout         = 20 * time.Second
)

// appleHVReadyUnit is a unit file that sets up the virtual serial device
// where when the VM is done configuring, it will send an ack
// so a listening host knows it can begin interacting with it
const appleHVReadyUnit = `[Unit]
Requires=dev-virtio\\x2dports-%s.device
After=remove-moby.service sshd.socket sshd.service
OnFailure=emergency.target
OnFailureJobMode=isolate
[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/bin/sh -c '/usr/bin/echo Ready | socat - VSOCK-CONNECT:2:1025'
[Install]
RequiredBy=default.target
`

type MacMachine struct {
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
	// ReadySocket tells host when vm is booted
	ReadySocket define.VMFile
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
	// The VFKit endpoint where we can interact with the VM
	Vfkit       vfkit.VfkitHelper
	LogPath     define.VMFile
	GvProxyPid  define.VMFile
	GvProxySock define.VMFile

	// Used at runtime for serializing write operations
	lock *lockfile.LockFile
}

// setGVProxyInfo sets the VM's gvproxy pid and socket files
func (m *MacMachine) setGVProxyInfo(runtimeDir string) error {
	gvProxyPid, err := define.NewMachineFile(filepath.Join(runtimeDir, "gvproxy.pid"), nil)
	if err != nil {
		return err
	}
	m.GvProxyPid = *gvProxyPid

	return sockets.SetSocket(&m.GvProxySock, filepath.Join(runtimeDir, "gvproxy.sock"), nil)
}

// setVfkitInfo stores the default devices, sets the vfkit endpoint, and
// locates/stores the path to the binary
func (m *MacMachine) setVfkitInfo(cfg *config.Config, readySocket define.VMFile) error {
	defaultDevices, err := getDefaultDevices(m.ImagePath.GetPath(), m.LogPath.GetPath(), readySocket.GetPath())
	if err != nil {
		return err
	}
	// Store VFKit stuffs
	vfkitPath, err := cfg.FindHelperBinary("vfkit", false)
	if err != nil {
		return err
	}
	vfkitBinaryPath, err := define.NewMachineFile(vfkitPath, nil)
	if err != nil {
		return err
	}

	m.Vfkit.VirtualMachine.Devices = defaultDevices
	m.Vfkit.Endpoint = defaultVFKitEndpoint
	m.Vfkit.VfkitBinaryPath = vfkitBinaryPath

	return nil
}

// addMountsToVM converts the volumes passed through the CLI to virtio-fs mounts
// and adds them to the machine
func (m *MacMachine) addMountsToVM(opts machine.InitOptions, virtiofsMnts *[]machine.VirtIoFs) error {
	var mounts []vmconfigs.Mount
	for _, volume := range opts.Volumes {
		source, target, _, readOnly, err := machine.ParseVolumeFromPath(volume)
		if err != nil {
			return err
		}
		mnt := machine.NewVirtIoFsMount(source, target, readOnly)
		*virtiofsMnts = append(*virtiofsMnts, mnt)
		mounts = append(mounts, mnt.ToMount())
	}
	m.Mounts = mounts

	return nil
}

func (m *MacMachine) Init(opts machine.InitOptions) (bool, error) {
	var (
		key          string
		virtiofsMnts []machine.VirtIoFs
		err          error
	)

	// cleanup half-baked files if init fails at any point
	callbackFuncs := machine.InitCleanup()
	defer callbackFuncs.CleanIfErr(&err)
	go callbackFuncs.CleanOnSignal()

	callbackFuncs.Add(m.ConfigPath.Delete)
	dataDir, err := machine.GetDataDir(define.AppleHvVirt)
	if err != nil {
		return false, err
	}
	cfg, err := config.Default()
	if err != nil {
		return false, err
	}

	dl, err := VirtualizationProvider().NewDownload(m.Name)
	if err != nil {
		return false, err
	}

	imagePath, strm, err := dl.AcquireVMImage(opts.ImagePath)
	if err != nil {
		return false, err
	}
	callbackFuncs.Add(imagePath.Delete)

	// Set the values for imagePath and strm
	m.ImagePath = *imagePath
	m.ImageStream = strm.String()

	logPath, err := define.NewMachineFile(filepath.Join(dataDir, fmt.Sprintf("%s.log", m.Name)), nil)
	if err != nil {
		return false, err
	}
	callbackFuncs.Add(logPath.Delete)

	m.LogPath = *logPath
	runtimeDir, err := m.getRuntimeDir()
	if err != nil {
		return false, err
	}

	if err := sockets.SetSocket(&m.ReadySocket, sockets.ReadySocketPath(runtimeDir, m.Name), nil); err != nil {
		return false, err
	}

	if err = m.setGVProxyInfo(runtimeDir); err != nil {
		return false, err
	}

	if err = m.setVfkitInfo(cfg, m.ReadySocket); err != nil {
		return false, err
	}

	m.IdentityPath = util.GetIdentityPath(m.Name)
	m.Rootful = opts.Rootful
	m.RemoteUsername = opts.Username

	m.UID = os.Getuid()

	sshPort, err := utils.GetRandomPort()
	if err != nil {
		return false, err
	}
	m.Port = sshPort

	if err = m.addMountsToVM(opts, &virtiofsMnts); err != nil {
		return false, err
	}

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

	logrus.Debugf("resizing disk to %d GiB", opts.DiskSize)
	if err = m.resizeDisk(strongunits.GiB(opts.DiskSize)); err != nil {
		return false, err
	}

	if err = m.writeConfig(); err != nil {
		return false, err
	}

	if len(opts.IgnitionPath) < 1 {
		key, err = machine.CreateSSHKeys(m.IdentityPath)
		if err != nil {
			return false, err
		}
		callbackFuncs.Add(m.removeSSHKeys)
	}

	builder := ignition.NewIgnitionBuilder(ignition.DynamicIgnition{
		Name:      opts.Username,
		Key:       key,
		VMName:    m.Name,
		VMType:    define.AppleHvVirt,
		TimeZone:  opts.TimeZone,
		WritePath: m.IgnitionFile.GetPath(),
		UID:       m.UID,
		Rootful:   m.Rootful,
	})

	if len(opts.IgnitionPath) > 0 {
		return false, builder.BuildWithIgnitionFile(opts.IgnitionPath)
	}

	if err := builder.GenerateIgnitionConfig(); err != nil {
		return false, err
	}

	builder.WithUnit(ignition.Unit{
		Enabled:  ignition.BoolToPtr(true),
		Name:     "ready.service",
		Contents: ignition.StrToPtr(fmt.Sprintf(appleHVReadyUnit, "vsock")),
	})
	builder.WithUnit(generateSystemDFilesForVirtiofsMounts(virtiofsMnts)...)

	// TODO Ignition stuff goes here
	err = builder.Build()
	callbackFuncs.Add(m.IgnitionFile.Delete)

	return err == nil, err
}

func (m *MacMachine) removeSSHKeys() error {
	if err := os.Remove(fmt.Sprintf("%s.pub", m.IdentityPath)); err != nil {
		logrus.Error(err)
	}
	return os.Remove(m.IdentityPath)
}

func (m *MacMachine) removeSystemConnections() error {
	return machine.RemoveConnections(m.Name, fmt.Sprintf("%s-root", m.Name))
}

func (m *MacMachine) Inspect() (*machine.InspectInfo, error) {
	vmState, err := m.Vfkit.State()
	if err != nil {
		return nil, err
	}

	podmanSocket, err := m.forwardSocketPath()
	if err != nil {
		return nil, err
	}

	ii := machine.InspectInfo{
		ConfigPath: m.ConfigPath,
		ConnectionInfo: machine.ConnectionConfig{
			PodmanSocket: podmanSocket,
			PodmanPipe:   nil,
		},
		Created: m.Created,
		Image: machine.ImageConfig{
			IgnitionFile: m.IgnitionFile,
			ImageStream:  m.ImageStream,
			ImagePath:    m.ImagePath,
		},
		LastUp: m.LastUp,
		Name:   m.Name,
		Resources: vmconfigs.ResourceConfig{
			CPUs:     m.CPUs,
			DiskSize: m.DiskSize,
			Memory:   m.Memory,
		},
		SSHConfig: m.SSHConfig,
		State:     vmState,
		Rootful:   m.Rootful,
	}
	return &ii, nil
}

// collectFilesToDestroy retrieves the files that will be destroyed by `Remove`
func (m *MacMachine) collectFilesToDestroy(opts machine.RemoveOptions) []string {
	files := []string{}
	if !opts.SaveKeys {
		files = append(files, m.IdentityPath, m.IdentityPath+".pub")
	}
	if !opts.SaveIgnition {
		files = append(files, m.IgnitionFile.GetPath())
	}

	if !opts.SaveImage {
		files = append(files, m.ImagePath.GetPath())
	}

	files = append(files, m.ConfigPath.GetPath())
	return files
}

func (m *MacMachine) Remove(name string, opts machine.RemoveOptions) (string, func() error, error) {
	var (
		files []string
	)

	m.lock.Lock()
	defer m.lock.Unlock()

	vmState, err := m.Vfkit.State()
	if err != nil {
		return "", nil, err
	}

	if vmState == define.Running {
		if !opts.Force {
			return "", nil, &machine.ErrVMRunningCannotDestroyed{Name: m.Name}
		}
		if err := m.Vfkit.Stop(true, true); err != nil {
			return "", nil, err
		}
		defer func() {
			if err := machine.CleanupGVProxy(m.GvProxyPid); err != nil {
				logrus.Error(err)
			}
		}()
	}

	files = m.collectFilesToDestroy(opts)

	confirmationMessage := "\nThe following files will be deleted:\n\n"
	for _, msg := range files {
		confirmationMessage += msg + "\n"
	}

	confirmationMessage += "\n"
	return confirmationMessage, func() error {
		machine.RemoveFilesAndConnections(files, m.Name, m.Name+"-root")
		// TODO We will need something like this for applehv too i think
		/*
			// Remove the HVSOCK for networking
			if err := m.NetworkHVSock.Remove(); err != nil {
				logrus.Errorf("unable to remove registry entry for %s: %q", m.NetworkHVSock.KeyName, err)
			}

			// Remove the HVSOCK for events
			if err := m.ReadyHVSock.Remove(); err != nil {
				logrus.Errorf("unable to remove registry entry for %s: %q", m.NetworkHVSock.KeyName, err)
			}
		*/
		return nil
	}, nil
}

func (m *MacMachine) writeConfig() error {
	b, err := json.MarshalIndent(m, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.ConfigPath.Path, b, 0644)
}

func (m *MacMachine) Set(name string, opts machine.SetOptions) ([]error, error) {
	var setErrors []error

	m.lock.Lock()
	defer m.lock.Unlock()

	vmState, err := m.State(false)
	if err != nil {
		return nil, err
	}
	if vmState != define.Stopped {
		return nil, machine.ErrWrongState
	}
	if cpus := opts.CPUs; cpus != nil {
		m.CPUs = *cpus
	}
	if mem := opts.Memory; mem != nil {
		m.Memory = *mem
	}
	if newSize := opts.DiskSize; newSize != nil {
		if *newSize < m.DiskSize {
			setErrors = append(setErrors, errors.New("new disk size smaller than existing disk size: cannot shrink disk size"))
		} else {
			m.DiskSize = *newSize
			if err := m.resizeDisk(strongunits.GiB(*opts.DiskSize)); err != nil {
				setErrors = append(setErrors, err)
			}
		}
	}
	if opts.USBs != nil {
		setErrors = append(setErrors, errors.New("changing USBs not supported for applehv machines"))
	}

	// Write the machine config to the filesystem
	err = m.writeConfig()
	setErrors = append(setErrors, err)
	switch len(setErrors) {
	case 0:
		return setErrors, nil
	case 1:
		return nil, setErrors[0]
	default:
		// Number of errors is 2 or more
		lastErr := setErrors[len(setErrors)-1]
		return setErrors[:len(setErrors)-1], lastErr
	}
}

func (m *MacMachine) SSH(name string, opts machine.SSHOptions) error {
	st, err := m.State(false)
	if err != nil {
		return err
	}
	if st != define.Running {
		return fmt.Errorf("vm %q is not running", m.Name)
	}
	username := opts.Username
	if username == "" {
		username = m.RemoteUsername
	}
	return machine.CommonSSH(username, m.IdentityPath, m.Name, m.Port, opts.Args)
}

// deleteIgnitionSocket retrieves the ignition socket, deletes it, and returns a
// pointer to the `VMFile`
func (m *MacMachine) deleteIgnitionSocket() (*define.VMFile, error) {
	ignitionSocket, err := m.getIgnitionSock()
	if err != nil {
		return nil, err
	}
	if err := ignitionSocket.Delete(); err != nil {
		return nil, err
	}
	return ignitionSocket, nil
}

// getIgnitionVsockDeviceAsCLI retrieves the ignition vsock device and converts
// it to a cmdline format
func getIgnitionVsockDeviceAsCLI(ignitionSocketPath string) ([]string, error) {
	ignitionVsockDevice, err := getIgnitionVsockDevice(ignitionSocketPath)
	if err != nil {
		return nil, err
	}
	// Convert the device into cli args
	ignitionVsockDeviceCLI, err := ignitionVsockDevice.ToCmdLine()
	if err != nil {
		return nil, err
	}
	return ignitionVsockDeviceCLI, nil
}

// getDebugDevicesCMDArgs retrieves the debug devices and converts them to a
// cmdline format
func getDebugDevicesCMDArgs() ([]string, error) {
	args := []string{}
	debugDevices, err := getDebugDevices()
	if err != nil {
		return nil, err
	}
	for _, debugDevice := range debugDevices {
		debugCli, err := debugDevice.ToCmdLine()
		if err != nil {
			return nil, err
		}
		args = append(args, debugCli...)
	}
	return args, nil
}

// getVfKitEndpointCMDArgs converts the vfkit endpoint to a cmdline format
func getVfKitEndpointCMDArgs(endpoint string) ([]string, error) {
	restEndpoint, err := vfRest.NewEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	return restEndpoint.ToCmdLine()
}

// addVolumesToVfKit adds the VM's mounts to vfkit's devices
func (m *MacMachine) addVolumesToVfKit() error {
	for _, vol := range m.Mounts {
		virtfsDevice, err := vfConfig.VirtioFsNew(vol.Source, vol.Tag)
		if err != nil {
			return err
		}
		m.Vfkit.VirtualMachine.Devices = append(m.Vfkit.VirtualMachine.Devices, virtfsDevice)
	}
	return nil
}

func (m *MacMachine) Start(name string, opts machine.StartOptions) error {
	var ignitionSocket *define.VMFile

	m.lock.Lock()
	defer m.lock.Unlock()

	st, err := m.State(false)
	if err != nil {
		return err
	}

	if st == define.Running {
		return machine.ErrVMAlreadyRunning
	}

	ioEater, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer ioEater.Close()

	// TODO handle returns from startHostNetworking
	forwardSock, forwardState, err := m.startHostNetworking()
	if err != nil {
		return err
	}

	// Add networking
	netDevice, err := vfConfig.VirtioNetNew("5a:94:ef:e4:0c:ee")
	if err != nil {
		return err
	}
	// Set user networking with gvproxy
	netDevice.SetUnixSocketPath(m.GvProxySock.GetPath())

	m.Vfkit.VirtualMachine.Devices = append(m.Vfkit.VirtualMachine.Devices, netDevice)

	if err := m.addVolumesToVfKit(); err != nil {
		return err
	}

	// To start the VM, we need to call vfkit

	logrus.Debugf("vfkit path is: %s", m.Vfkit.VfkitBinaryPath.Path)
	cmd, err := m.Vfkit.VirtualMachine.Cmd(m.Vfkit.VfkitBinaryPath.Path)
	if err != nil {
		return err
	}

	vfkitEndpointArgs, err := getVfKitEndpointCMDArgs(m.Vfkit.Endpoint)
	if err != nil {
		return err
	}
	cmd.Args = append(cmd.Args, vfkitEndpointArgs...)

	firstBoot, err := m.isFirstBoot()
	if err != nil {
		return err
	}

	if firstBoot {
		// If this is the first boot of the vm, we need to add the vsock
		// device to vfkit so we can inject the ignition file
		ignitionSocket, err = m.deleteIgnitionSocket()
		if err != nil {
			return err
		}

		ignitionVsockDeviceCLI, err := getIgnitionVsockDeviceAsCLI(ignitionSocket.GetPath())
		if err != nil {
			return err
		}
		cmd.Args = append(cmd.Args, ignitionVsockDeviceCLI...)
	}

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		debugDevArgs, err := getDebugDevicesCMDArgs()
		if err != nil {
			return err
		}
		cmd.Args = append(cmd.Args, debugDevArgs...)
		cmd.Args = append(cmd.Args, "--gui") // add command line switch to pop the gui open
	}

	cmd.ExtraFiles = []*os.File{ioEater, ioEater, ioEater}

	readSocketBaseDir := filepath.Dir(m.ReadySocket.GetPath())
	if err := os.MkdirAll(readSocketBaseDir, 0755); err != nil {
		return err
	}

	if firstBoot {
		logrus.Debug("first boot detected")
		logrus.Debugf("serving ignition file over %s", ignitionSocket.GetPath())
		go func() {
			if err := m.serveIgnitionOverSock(ignitionSocket); err != nil {
				logrus.Error(err)
			}
			logrus.Debug("ignition vsock server exited")
		}()
	}

	if err := m.ReadySocket.Delete(); err != nil {
		return err
	}

	logrus.Debugf("listening for ready on: %s", m.ReadySocket.GetPath())
	readyListen, err := net.Listen("unix", m.ReadySocket.GetPath())
	if err != nil {
		return err
	}

	logrus.Debug("waiting for ready notification")
	readyChan := make(chan error)
	go sockets.ListenAndWaitOnSocket(readyChan, readyListen)

	if err := cmd.Start(); err != nil {
		return err
	}

	processErrChan := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		defer close(processErrChan)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if err := checkProcessRunning("vfkit", cmd.Process.Pid); err != nil {
				processErrChan <- err
				return
			}
			// lets poll status every half second
			time.Sleep(500 * time.Millisecond)
		}
	}()

	// wait for either socket or to be ready or process to have exited
	select {
	case err := <-processErrChan:
		if err != nil {
			return err
		}
	case err := <-readyChan:
		if err != nil {
			return err
		}
	}

	logrus.Debug("ready notification received")
	machine.WaitAPIAndPrintInfo(
		forwardState,
		m.Name,
		findClaimHelper(),
		forwardSock,
		opts.NoInfo,
		m.isIncompatible(),
		m.Rootful,
	)
	return nil
}

func (m *MacMachine) State(_ bool) (define.Status, error) {
	vmStatus, err := m.Vfkit.State()
	if err != nil {
		return "", err
	}
	return vmStatus, nil
}

func (m *MacMachine) Stop(name string, opts machine.StopOptions) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	vmState, err := m.State(false)
	if err != nil {
		return err
	}

	if vmState != define.Running {
		return nil
	}

	defer func() {
		if err := machine.CleanupGVProxy(m.GvProxyPid); err != nil {
			logrus.Error(err)
		}
	}()

	return m.Vfkit.Stop(false, true)
}

// getVMConfigPath is a simple wrapper for getting the fully-qualified
// path of the vm json config file.  It should be used to get conformity
func getVMConfigPath(configDir, vmName string) string {
	return filepath.Join(configDir, fmt.Sprintf("%s.json", vmName))
}

func (m *MacMachine) loadFromFile() (*MacMachine, error) {
	if len(m.Name) < 1 {
		return nil, errors.New("encountered machine with no name")
	}

	jsonPath, err := m.jsonConfigPath()
	if err != nil {
		return nil, err
	}

	mm, err := loadMacMachineFromJSON(jsonPath)
	if err != nil {
		return nil, err
	}

	lock, err := machine.GetLock(mm.Name, vmtype)
	if err != nil {
		return nil, err
	}
	mm.lock = lock

	return mm, nil
}

func loadMacMachineFromJSON(fqConfigPath string) (*MacMachine, error) {
	b, err := os.ReadFile(fqConfigPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			name := strings.TrimSuffix(filepath.Base(fqConfigPath), ".json")
			return nil, fmt.Errorf("%s: %w", name, machine.ErrNoSuchVM)
		}
		return nil, err
	}
	mm := new(MacMachine)
	if err := json.Unmarshal(b, mm); err != nil {
		return nil, err
	}
	return mm, nil
}

func (m *MacMachine) jsonConfigPath() (string, error) {
	configDir, err := machine.GetConfDir(define.AppleHvVirt)
	if err != nil {
		return "", err
	}
	return getVMConfigPath(configDir, m.Name), nil
}

func getVMInfos() ([]*machine.ListResponse, error) {
	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return nil, err
	}

	var listed []*machine.ListResponse

	if err = filepath.WalkDir(vmConfigDir, func(path string, d fs.DirEntry, err error) error {
		vm := new(MacMachine)
		if strings.HasSuffix(d.Name(), ".json") {
			fullPath := filepath.Join(vmConfigDir, d.Name())
			b, err := os.ReadFile(fullPath)
			if err != nil {
				return err
			}
			err = json.Unmarshal(b, vm)
			if err != nil {
				return err
			}
			listEntry := new(machine.ListResponse)

			listEntry.Name = vm.Name
			listEntry.Stream = vm.ImageStream
			listEntry.VMType = define.AppleHvVirt.String()
			listEntry.CPUs = vm.CPUs
			listEntry.Memory = vm.Memory * units.MiB
			listEntry.DiskSize = vm.DiskSize * units.GiB
			listEntry.Port = vm.Port
			listEntry.RemoteUsername = vm.RemoteUsername
			listEntry.IdentityPath = vm.IdentityPath
			listEntry.CreatedAt = vm.Created
			listEntry.Starting = vm.Starting

			if listEntry.CreatedAt.IsZero() {
				listEntry.CreatedAt = time.Now()
				vm.Created = time.Now()
				if err := vm.writeConfig(); err != nil {
					return err
				}
			}

			vmState, err := vm.State(false)
			if err != nil {
				return err
			}
			listEntry.Running = vmState == define.Running
			listEntry.LastUp = vm.LastUp

			listed = append(listed, listEntry)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return listed, err
}

// setupStartHostNetworkingCmd generates the cmd that will be used to start the
// host networking. Includes the ssh port, gvproxy pid file, gvproxy socket, and
// a debug flag depending on the logrus log level
func (m *MacMachine) setupStartHostNetworkingCmd() (gvproxy.GvproxyCommand, string, machine.APIForwardingState) {
	cmd := gvproxy.NewGvproxyCommand()
	cmd.SSHPort = m.Port
	cmd.PidFile = m.GvProxyPid.GetPath()
	cmd.AddVfkitSocket(fmt.Sprintf("unixgram://%s", m.GvProxySock.GetPath()))
	cmd.Debug = logrus.IsLevelEnabled(logrus.DebugLevel)

	cmd, forwardSock, state := m.setupAPIForwarding(cmd)
	if cmd.Debug {
		logrus.Debug(cmd.ToCmdline())
	}

	return cmd, forwardSock, state
}

func (m *MacMachine) startHostNetworking() (string, machine.APIForwardingState, error) {
	var (
		forwardSock string
		state       machine.APIForwardingState
	)

	// TODO This should probably be added to startHostNetworking everywhere
	// GvProxy does not clean up after itself
	if err := m.GvProxySock.Delete(); err != nil {
		b, err := m.GvProxyPid.Read()
		if err != nil {
			return "", machine.NoForwarding, err
		}
		pid, err := strconv.Atoi(string(b))
		if err != nil {
			return "", 0, err
		}
		gvProcess, err := os.FindProcess(pid)
		if err != nil {
			return "", 0, err
		}
		// shoot it with a signal 0 and see if it is active
		err = gvProcess.Signal(syscall.Signal(0))
		if err == nil {
			return "", 0, fmt.Errorf("gvproxy process %s already running", string(b))
		}
		if err := m.GvProxySock.Delete(); err != nil {
			return "", 0, err
		}
	}
	cfg, err := config.Default()
	if err != nil {
		return "", machine.NoForwarding, err
	}

	gvproxyBinary, err := cfg.FindHelperBinary("gvproxy", false)
	if err != nil {
		return "", 0, err
	}

	logrus.Debugf("gvproxy binary being used: %s", gvproxyBinary)

	cmd, forwardSock, state := m.setupStartHostNetworkingCmd()
	c := cmd.Cmd(gvproxyBinary)
	if err := c.Start(); err != nil {
		return "", 0, fmt.Errorf("unable to execute: %q: %w", cmd.ToCmdline(), err)
	}

	// We need to wait and make sure gvproxy is in fact running
	// before continuing
	for i := 0; i < 10; i++ {
		_, err := os.Stat(m.GvProxySock.GetPath())
		if err == nil {
			break
		}
		if err := checkProcessRunning("gvproxy", c.Process.Pid); err != nil {
			// gvproxy is no longer running
			return "", 0, err
		}
		logrus.Debugf("gvproxy unixgram socket %q not found: %v", m.GvProxySock.GetPath(), err)
		// Sleep for 1/2 second
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		// I guess we would also check the pidfile and look to see if it is running
		// to?
		return "", 0, fmt.Errorf("unable to verify gvproxy is running")
	}
	return forwardSock, state, nil
}

// checkProcessRunning checks non blocking if the pid exited
// returns nil if process is running otherwise an error if not
func checkProcessRunning(processName string, pid int) error {
	var status syscall.WaitStatus
	pid, err := syscall.Wait4(pid, &status, syscall.WNOHANG, nil)
	if err != nil {
		return fmt.Errorf("failed to read %s process status: %w", processName, err)
	}
	if pid > 0 {
		// child exited
		return fmt.Errorf("%s exited unexpectedly with exit code %d", processName, status.ExitStatus())
	}
	return nil
}

func (m *MacMachine) setupAPIForwarding(cmd gvproxy.GvproxyCommand) (gvproxy.GvproxyCommand, string, machine.APIForwardingState) {
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

	link, err := m.userGlobalSocketLink()
	if err != nil {
		return cmd, socket.GetPath(), machine.MachineLocal
	}

	if !dockerClaimSupported() {
		return cmd, socket.GetPath(), machine.ClaimUnsupported
	}

	if !dockerClaimHelperInstalled() {
		return cmd, socket.GetPath(), machine.NotInstalled
	}

	if !alreadyLinked(socket.GetPath(), link) {
		if checkSockInUse(link) {
			return cmd, socket.GetPath(), machine.MachineLocal
		}

		_ = os.Remove(link)
		if err = os.Symlink(socket.GetPath(), link); err != nil {
			logrus.Warnf("could not create user global API forwarding link: %s", err.Error())
			return cmd, socket.GetPath(), machine.MachineLocal
		}
	}

	if !alreadyLinked(link, dockerSock) {
		if checkSockInUse(dockerSock) {
			return cmd, socket.GetPath(), machine.MachineLocal
		}

		if !claimDockerSock() {
			logrus.Warn("podman helper is installed, but was not able to claim the global docker sock")
			return cmd, socket.GetPath(), machine.MachineLocal
		}
	}

	return cmd, "", machine.MachineLocal
}

func (m *MacMachine) dockerSock() (string, error) {
	dd, err := machine.GetDataDir(define.AppleHvVirt)
	if err != nil {
		return "", err
	}
	return filepath.Join(dd, "podman.sock"), nil
}

func (m *MacMachine) forwardSocketPath() (*define.VMFile, error) {
	sockName := "podman.sock"
	path, err := machine.GetDataDir(define.AppleHvVirt)
	if err != nil {
		return nil, fmt.Errorf("Resolving data dir: %s", err.Error())
	}
	return define.NewMachineFile(filepath.Join(path, sockName), &sockName)
}

// resizeDisk uses os truncate to resize (only larger) a raw disk.  the input size
// is assumed GiB
func (m *MacMachine) resizeDisk(newSize strongunits.GiB) error {
	if uint64(newSize) < m.DiskSize {
		// TODO this error needs to be changed to the common error.  would do now but the PR for the common
		// error has not merged
		return fmt.Errorf("invalid disk size %d: new disk must be larger than %dGB", newSize, m.DiskSize)
	}
	logrus.Debugf("resizing %s to %d bytes", m.ImagePath.GetPath(), newSize.ToBytes())
	// seems like os.truncate() is not very performant with really large files
	// so exec'ing out to the command truncate
	size := fmt.Sprintf("%dG", newSize)
	c := exec.Command("truncate", "-s", size, m.ImagePath.GetPath())
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		c.Stderr = os.Stderr
		c.Stdout = os.Stdout
	}
	return c.Run()
}

// isFirstBoot returns a bool reflecting if the machine has been booted before
func (m *MacMachine) isFirstBoot() (bool, error) {
	never, err := time.Parse(time.RFC3339, "0001-01-01T00:00:00Z")
	if err != nil {
		return false, err
	}
	return m.LastUp == never, nil
}

func (m *MacMachine) getIgnitionSock() (*define.VMFile, error) {
	dataDir, err := machine.GetDataDir(define.AppleHvVirt)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	return define.NewMachineFile(filepath.Join(dataDir, ignitionSocketName), nil)
}

func (m *MacMachine) getRuntimeDir() (string, error) {
	tmpDir, ok := os.LookupEnv("TMPDIR")
	if !ok {
		tmpDir = "/tmp"
	}
	rtd := filepath.Join(tmpDir, "podman")
	logrus.Debugf("creating runtimeDir: %s", rtd)
	if err := os.MkdirAll(rtd, 0755); err != nil {
		return "", err
	}

	return rtd, nil
}

func (m *MacMachine) userGlobalSocketLink() (string, error) {
	path, err := machine.GetDataDir(define.AppleHvVirt)
	if err != nil {
		logrus.Errorf("Resolving data dir: %s", err.Error())
		return "", err
	}
	// User global socket is located in parent directory of machine dirs (one per user)
	return filepath.Join(filepath.Dir(path), "podman.sock"), err
}

func (m *MacMachine) isIncompatible() bool {
	return m.UID == -1
}

func generateSystemDFilesForVirtiofsMounts(mounts []machine.VirtIoFs) []ignition.Unit {
	// mounting in fcos with virtiofs is a bit of a dance.  we need a unit file for the mount, a unit file
	// for automatic mounting on boot, and a "preparatory" service file that disables FCOS security, performs
	// the mkdir of the mount point, and then re-enables security.  This must be done for each mount.

	var unitFiles []ignition.Unit
	for _, mnt := range mounts {
		// Here we are looping the mounts and for each mount, we are adding two unit files
		// for virtiofs.  One unit file is the mount itself and the second is to automount it
		// on boot.
		autoMountUnit := `[Automount]
Where=%s
[Install]
WantedBy=multi-user.target

[Unit]
Description=Mount virtiofs volume %s
`
		mountUnit := `[Mount]
What=%s
Where=%s
Type=virtiofs

[Install]
WantedBy=multi-user.target
`
		virtiofsAutomount := ignition.Unit{
			Enabled:  ignition.BoolToPtr(true),
			Name:     fmt.Sprintf("%s.automount", mnt.Tag),
			Contents: ignition.StrToPtr(fmt.Sprintf(autoMountUnit, mnt.Target, mnt.Target)),
		}
		virtiofsMount := ignition.Unit{
			Enabled:  ignition.BoolToPtr(true),
			Name:     fmt.Sprintf("%s.mount", mnt.Tag),
			Contents: ignition.StrToPtr(fmt.Sprintf(mountUnit, mnt.Tag, mnt.Target)),
		}

		// This "unit" simulates something like systemctl enable virtiofs-mount-prepare@
		enablePrep := ignition.Unit{
			Enabled: ignition.BoolToPtr(true),
			Name:    fmt.Sprintf("virtiofs-mount-prepare@%s.service", mnt.Tag),
		}

		unitFiles = append(unitFiles, virtiofsAutomount, virtiofsMount, enablePrep)
	}

	// mount prep is a way to workaround the FCOS limitation of creating directories
	// at the rootfs / and then mounting to them.
	mountPrep := `
[Unit]
Description=Allow virtios to mount to /
DefaultDependencies=no
ConditionPathExists=!%f

[Service]
Type=oneshot
ExecStartPre=chattr -i /
ExecStart=mkdir -p '%f'
ExecStopPost=chattr +i /

[Install]
WantedBy=remote-fs.target
`
	virtioFSChattr := ignition.Unit{
		Contents: ignition.StrToPtr(mountPrep),
		Name:     "virtiofs-mount-prepare@.service",
	}
	unitFiles = append(unitFiles, virtioFSChattr)

	return unitFiles
}
