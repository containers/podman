//go:build darwin
// +build darwin

package applehv

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/podman/v4/utils"
	vfConfig "github.com/crc-org/vfkit/pkg/config"
	vfRest "github.com/crc-org/vfkit/pkg/rest"
	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
)

var (
	// vmtype refers to qemu (vs libvirt, krun, etc).
	vmtype = machine.AppleHvVirt
)

const (
	dockerSock           = "/var/run/docker.sock"
	dockerConnectTimeout = 5 * time.Second
	apiUpTimeout         = 20 * time.Second
)

// VfkitHelper describes the use of vfkit: cmdline and endpoint
type VfkitHelper struct {
	LogLevel        logrus.Level
	Endpoint        string
	VfkitBinaryPath *machine.VMFile
	VirtualMachine  *vfConfig.VirtualMachine
}

type MacMachine struct {
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
	// The VFKit endpoint where we can interact with the VM
	Vfkit       VfkitHelper
	LogPath     machine.VMFile
	GvProxyPid  machine.VMFile
	GvProxySock machine.VMFile
}

func (m *MacMachine) Init(opts machine.InitOptions) (bool, error) {
	var (
		key          string
		mounts       []machine.Mount
		virtiofsMnts []machine.VirtIoFs
	)
	dataDir, err := machine.GetDataDir(machine.AppleHvVirt)
	if err != nil {
		return false, err
	}
	cfg, err := config.Default()
	if err != nil {
		return false, err
	}

	// Acquire the image
	switch opts.ImagePath {
	case machine.Testing.String(), machine.Next.String(), machine.Stable.String(), "":
		g, err := machine.NewGenericDownloader(machine.HyperVVirt, opts.Name, opts.ImagePath)
		if err != nil {
			return false, err
		}

		imagePath, err := machine.NewMachineFile(g.Get().GetLocalUncompressedFile(dataDir), nil)
		if err != nil {
			return false, err
		}
		m.ImagePath = *imagePath
	default:
		// The user has provided an alternate image which can be a file path
		// or URL.
		m.ImageStream = "custom"
		g, err := machine.NewGenericDownloader(vmtype, m.Name, opts.ImagePath)
		if err != nil {
			return false, err
		}
		imagePath, err := machine.NewMachineFile(g.Get().LocalUncompressedFile, nil)
		if err != nil {
			return false, err
		}
		m.ImagePath = *imagePath
		if err := machine.DownloadImage(g); err != nil {
			return false, err
		}
	}

	logPath, err := machine.NewMachineFile(filepath.Join(dataDir, fmt.Sprintf("%s.log", m.Name)), nil)
	if err != nil {
		return false, err
	}
	m.LogPath = *logPath
	runtimeDir, err := m.getRuntimeDir()
	if err != nil {
		return false, err
	}

	readySocket, err := machine.NewMachineFile(filepath.Join(runtimeDir, fmt.Sprintf("%s_ready.sock", m.Name)), nil)
	if err != nil {
		return false, err
	}

	gvProxyPid, err := machine.NewMachineFile(filepath.Join(runtimeDir, "gvproxy.pid"), nil)
	if err != nil {
		return false, err
	}

	gvProxySock, err := machine.NewMachineFile(filepath.Join(runtimeDir, "gvproxy.sock"), nil)
	if err != nil {
		return false, err
	}

	defaultDevices, err := getDefaultDevices(m.ImagePath.GetPath(), m.LogPath.GetPath(), readySocket.GetPath())
	if err != nil {
		return false, err
	}
	// Store VFKit stuffs
	vfkitPath, err := cfg.FindHelperBinary("vfkit", false)
	if err != nil {
		return false, err
	}
	vfkitBinaryPath, err := machine.NewMachineFile(vfkitPath, nil)
	if err != nil {
		return false, err
	}

	m.ReadySocket = *readySocket
	m.GvProxyPid = *gvProxyPid
	m.GvProxySock = *gvProxySock
	m.Vfkit.VirtualMachine.Devices = defaultDevices
	m.Vfkit.Endpoint = defaultVFKitEndpoint
	m.Vfkit.VfkitBinaryPath = vfkitBinaryPath

	m.IdentityPath = util.GetIdentityPath(m.Name)
	m.Rootful = opts.Rootful
	m.RemoteUsername = opts.Username

	m.UID = os.Getuid()

	sshPort, err := utils.GetRandomPort()
	if err != nil {
		return false, err
	}
	m.Port = sshPort

	for _, volume := range opts.Volumes {
		source, target, _, readOnly, err := machine.ParseVolumeFromPath(volume)
		if err != nil {
			return false, err
		}
		mnt := machine.NewVirtIoFsMount(source, target, readOnly)
		virtiofsMnts = append(virtiofsMnts, mnt)
		mounts = append(mounts, mnt.ToMount())
	}
	m.Mounts = mounts

	if len(opts.IgnitionPath) < 1 {
		// TODO localhost needs to be restored here
		uri := machine.SSHRemoteConnection.MakeSSHURL("localhost", fmt.Sprintf("/run/user/%d/podman/podman.sock", m.UID), strconv.Itoa(m.Port), m.RemoteUsername)
		uriRoot := machine.SSHRemoteConnection.MakeSSHURL("localhost", "/run/podman/podman.sock", strconv.Itoa(m.Port), "root")
		identity := m.IdentityPath

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

	// Until the disk resize can be fixed, we ignore it
	if err := m.resizeDisk(opts.DiskSize); err != nil && !errors.Is(err, define.ErrNotImplemented) {
		return false, err
	}

	if err := m.writeConfig(); err != nil {
		return false, err
	}

	if len(opts.IgnitionPath) < 1 {
		var err error
		key, err = machine.CreateSSHKeys(m.IdentityPath)
		if err != nil {
			return false, err
		}
	}

	if len(opts.IgnitionPath) > 0 {
		inputIgnition, err := os.ReadFile(opts.IgnitionPath)
		if err != nil {
			return false, err
		}
		return false, os.WriteFile(m.IgnitionFile.GetPath(), inputIgnition, 0644)
	}
	// TODO Ignition stuff goes here
	// Write the ignition file

	ign := machine.DynamicIgnition{
		Name:      opts.Username,
		Key:       key,
		VMName:    m.Name,
		VMType:    machine.AppleHvVirt,
		TimeZone:  opts.TimeZone,
		WritePath: m.IgnitionFile.GetPath(),
		UID:       m.UID,
		Rootful:   m.Rootful,
	}

	if err := ign.GenerateIgnitionConfig(); err != nil {
		return false, err
	}

	// ready is a unit file that sets up the virtual serial device
	// where when the VM is done configuring, it will send an ack
	// so a listening host knows it can being interacting with it
	ready := `[Unit]
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
	readyUnit := machine.Unit{
		Enabled:  machine.BoolToPtr(true),
		Name:     "ready.service",
		Contents: machine.StrToPtr(fmt.Sprintf(ready, "vsock")),
	}
	virtiofsUnits := generateSystemDFilesForVirtiofsMounts(virtiofsMnts)
	ign.Cfg.Systemd.Units = append(ign.Cfg.Systemd.Units, readyUnit)
	ign.Cfg.Systemd.Units = append(ign.Cfg.Systemd.Units, virtiofsUnits...)

	if err := ign.Write(); err != nil {
		return false, err
	}

	return true, nil
}

func (m *MacMachine) Inspect() (*machine.InspectInfo, error) {
	vmState, err := m.Vfkit.state()
	if err != nil {
		return nil, err
	}
	ii := machine.InspectInfo{
		ConfigPath: m.ConfigPath,
		ConnectionInfo: machine.ConnectionConfig{
			PodmanSocket: nil,
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
		Resources: machine.ResourceConfig{
			CPUs:     m.CPUs,
			DiskSize: m.DiskSize,
			Memory:   m.Memory,
		},
		SSHConfig: m.SSHConfig,
		State:     vmState,
	}
	return &ii, nil
}

func (m *MacMachine) Remove(name string, opts machine.RemoveOptions) (string, func() error, error) {
	var (
		files []string
	)

	vmState, err := m.Vfkit.state()
	if err != nil {
		return "", nil, err
	}
	if vmState == machine.Running {
		if !opts.Force {
			return "", nil, fmt.Errorf("invalid state: %s is running", m.Name)
		}
		if err := m.Vfkit.stop(true, true); err != nil {
			return "", nil, err
		}
	}

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
		if err := machine.RemoveConnections(m.Name); err != nil {
			logrus.Error(err)
		}
		if err := machine.RemoveConnections(m.Name + "-root"); err != nil {
			logrus.Error(err)
		}

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
	vmState, err := m.State(false)
	if err != nil {
		return nil, err
	}
	if vmState != machine.Stopped {
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
			if err := m.resizeDisk(*opts.DiskSize); err != nil {
				setErrors = append(setErrors, err)
			}
		}
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
	if st != machine.Running {
		return fmt.Errorf("vm %q is not running", m.Name)
	}
	username := opts.Username
	if username == "" {
		username = m.RemoteUsername
	}
	return machine.CommonSSH(username, m.IdentityPath, m.Name, m.Port, opts.Args)
}

func (m *MacMachine) Start(name string, opts machine.StartOptions) error {
	var ignitionSocket *machine.VMFile

	st, err := m.State(false)
	if err != nil {
		return err
	}
	if st == machine.Running {
		return machine.ErrVMAlreadyRunning
	}

	ioEater, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer ioEater.Close()

	// TODO handle returns from startHostNetworking
	forwardSock, forwardState, err := m.startHostNetworking(ioEater)
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

	for _, vol := range m.Mounts {
		virtfsDevice, err := vfConfig.VirtioFsNew(vol.Source, vol.Tag)
		if err != nil {
			return err
		}
		m.Vfkit.VirtualMachine.Devices = append(m.Vfkit.VirtualMachine.Devices, virtfsDevice)
	}

	// To start the VM, we need to call vfkit

	cmd, err := m.Vfkit.VirtualMachine.Cmd(m.Vfkit.VfkitBinaryPath.Path)
	if err != nil {
		return err
	}

	restEndpoint, err := vfRest.NewEndpoint(m.Vfkit.Endpoint)
	if err != nil {
		return err
	}
	restArgs, err := restEndpoint.ToCmdLine()
	if err != nil {
		return err
	}
	cmd.Args = append(cmd.Args, restArgs...)

	firstBoot, err := m.isFirstBoot()
	if err != nil {
		return err
	}
	if firstBoot {
		// If this is the first boot of the vm, we need to add the vsock
		// device to vfkit so we can inject the ignition file
		ignitionSocket, err = m.getIgnitionSock()
		if err != nil {
			return err
		}
		if err := ignitionSocket.Delete(); err != nil {
			return err
		}
		ignitionVsockDevice, err := getIgnitionVsockDevice(ignitionSocket.GetPath())
		if err != nil {
			return err
		}
		// Convert the device into cli args
		ignitionVsockDeviceCli, err := ignitionVsockDevice.ToCmdLine()
		if err != nil {
			return err
		}
		cmd.Args = append(cmd.Args, ignitionVsockDeviceCli...)
	}
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		debugDevices, err := getDebugDevices()
		if err != nil {
			return err
		}
		for _, debugDevice := range debugDevices {
			debugCli, err := debugDevice.ToCmdLine()
			if err != nil {
				return err
			}
			cmd.Args = append(cmd.Args, debugCli...)
		}
		cmd.Args = append(cmd.Args, "--gui") // add command line switch to pop the gui open
	}
	cmd.ExtraFiles = []*os.File{ioEater, ioEater, ioEater}
	fmt.Println(cmd.Args)

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
	var conn net.Conn
	readyChan := make(chan error)
	go func() {
		conn, err = readyListen.Accept()
		if err != nil {
			logrus.Error(err)
		}
		_, err = bufio.NewReader(conn).ReadString('\n')
		readyChan <- err
	}()
	if err := cmd.Start(); err != nil {
		return err
	}

	err = <-readyChan
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			logrus.Error(closeErr)
		}
	}()
	if err != nil {
		return err
	}
	logrus.Debug("ready notification received")
	m.waitAPIAndPrintInfo(forwardState, forwardSock, opts.NoInfo)
	return nil
}

func (m *MacMachine) State(_ bool) (machine.Status, error) {
	vmStatus, err := m.Vfkit.state()
	if err != nil {
		return "", err
	}
	return vmStatus, nil
}

func (m *MacMachine) Stop(name string, opts machine.StopOptions) error {
	vmState, err := m.State(false)
	if err != nil {
		return err
	}
	if vmState != machine.Running {
		return machine.ErrWrongState
	}

	defer func() {
		// I think "soft" errors here is OK
		gvPid, err := m.GvProxyPid.Read()
		if err != nil {
			logrus.Error(fmt.Errorf("unable to read gvproxy pid file %s: %v", m.GvProxyPid.GetPath(), err))
			return
		}
		proxyPid, err := strconv.Atoi(string(gvPid))
		if err != nil {
			logrus.Error(fmt.Errorf("unable to convert pid to integer: %v", err))
			return
		}
		proxyProc, err := os.FindProcess(proxyPid)
		if proxyProc == nil && err != nil {
			logrus.Error("unable to find process: %v", err)
			return
		}
		if err := proxyProc.Kill(); err != nil {
			logrus.Error("unable to kill gvproxy: %v", err)
			return
		}
		// gvproxy does not clean up its pid file on exit
		if err := m.GvProxyPid.Delete(); err != nil {
			logrus.Error("unable to delete gvproxy pid file: %v", err)
		}
	}()

	return m.Vfkit.stop(false, true)
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
	mm := MacMachine{}

	if err := loadMacMachineFromJSON(jsonPath, &mm); err != nil {
		return nil, err
	}
	return &mm, nil
}

func loadMacMachineFromJSON(fqConfigPath string, macMachine *MacMachine) error {
	b, err := os.ReadFile(fqConfigPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%q: %w", fqConfigPath, machine.ErrNoSuchVM)
		}
		return err
	}
	return json.Unmarshal(b, macMachine)
}

func (m *MacMachine) jsonConfigPath() (string, error) {
	configDir, err := machine.GetConfDir(machine.AppleHvVirt)
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
			listEntry.VMType = machine.AppleHvVirt.String()
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
			listEntry.Running = vmState == machine.Running

			if !vm.LastUp.IsZero() { // this means we have already written a time to the config
				listEntry.LastUp = vm.LastUp
			} else { // else we just created the machine AKA last up = created time
				listEntry.LastUp = vm.Created
				vm.LastUp = listEntry.LastUp
				if err := vm.writeConfig(); err != nil {
					return err
				}
			}

			listed = append(listed, listEntry)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return listed, err
}

func (m *MacMachine) startHostNetworking(ioEater *os.File) (string, machine.APIForwardingState, error) {
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
	attr := new(os.ProcAttr)
	gvproxy, err := cfg.FindHelperBinary("gvproxy", false)
	if err != nil {
		return "", 0, err
	}

	attr.Files = []*os.File{ioEater, ioEater, ioEater}
	cmd := []string{gvproxy}
	// Add the ssh port
	cmd = append(cmd, []string{"-ssh-port", fmt.Sprintf("%d", m.Port)}...)
	// Add pid file
	cmd = append(cmd, "-pid-file", m.GvProxyPid.GetPath())
	// Add vfkit proxy listen
	cmd = append(cmd, "-listen-vfkit", fmt.Sprintf("unixgram://%s", m.GvProxySock.GetPath()))
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

// AppleHVSSH is a temporary function for applehv until we decide how the networking will work
// for certain.
func AppleHVSSH(username, identityPath, name string, sshPort int, inputArgs []string) error {
	sshDestination := username + "@192.168.64.2"
	port := strconv.Itoa(sshPort)

	args := []string{"-i", identityPath, "-p", port, sshDestination,
		"-o", "IdentitiesOnly=yes",
		"-o", "StrictHostKeyChecking=no", "-o", "LogLevel=ERROR", "-o", "SetEnv=LC_ALL="}
	if len(inputArgs) > 0 {
		args = append(args, inputArgs...)
	} else {
		fmt.Printf("Connecting to vm %s. To close connection, use `~.` or `exit`\n", name)
	}

	cmd := exec.Command("ssh", args...)
	logrus.Debugf("Executing: ssh %v\n", args)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}
func (m *MacMachine) setupAPIForwarding(cmd []string) ([]string, string, machine.APIForwardingState) {
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
	dd, err := machine.GetDataDir(machine.AppleHvVirt)
	if err != nil {
		return "", err
	}
	return filepath.Join(dd, "podman.sock"), nil
}

func (m *MacMachine) forwardSocketPath() (*machine.VMFile, error) {
	sockName := "podman.sock"
	path, err := machine.GetDataDir(machine.AppleHvVirt)
	if err != nil {
		return nil, fmt.Errorf("Resolving data dir: %s", err.Error())
	}
	return machine.NewMachineFile(filepath.Join(path, sockName), &sockName)
}

func (m *MacMachine) resizeDisk(newSize uint64) error {
	// TODO truncating is not enough; we may not be able to support resizing with raw image?
	// Leaving for now but returning an unimplemented error

	//if newSize < m.DiskSize {
	//	return fmt.Errorf("invalid disk size %d: new disk must be larger than %dGB", newSize, m.DiskSize)
	//}
	//return os.Truncate(m.ImagePath.GetPath(), int64(newSize))
	return define.ErrNotImplemented
}

// isFirstBoot returns a bool reflecting if the machine has been booted before
func (m *MacMachine) isFirstBoot() (bool, error) {
	never, err := time.Parse(time.RFC3339, "0001-01-01T00:00:00Z")
	if err != nil {
		return false, err
	}
	return m.LastUp == never, nil
}

func (m *MacMachine) getIgnitionSock() (*machine.VMFile, error) {
	dataDir, err := machine.GetDataDir(machine.AppleHvVirt)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	return machine.NewMachineFile(filepath.Join(dataDir, ignitionSocketName), nil)
}

func (m *MacMachine) getRuntimeDir() (string, error) {
	tmpDir, ok := os.LookupEnv("TMPDIR")
	if !ok {
		tmpDir = "/tmp"
	}
	return filepath.Join(tmpDir, "podman"), nil
}

func (m *MacMachine) userGlobalSocketLink() (string, error) {
	path, err := machine.GetDataDir(machine.AppleHvVirt)
	if err != nil {
		logrus.Errorf("Resolving data dir: %s", err.Error())
		return "", err
	}
	// User global socket is located in parent directory of machine dirs (one per user)
	return filepath.Join(filepath.Dir(path), "podman.sock"), err
}

func (m *MacMachine) waitAPIAndPrintInfo(forwardState machine.APIForwardingState, forwardSock string, noInfo bool) {
	suffix := ""
	if m.Name != machine.DefaultMachineName {
		suffix = " " + m.Name
	}

	if m.isIncompatible() {
		fmt.Fprintf(os.Stderr, "\n!!! ACTION REQUIRED: INCOMPATIBLE MACHINE !!!\n")

		fmt.Fprintf(os.Stderr, "\nThis machine was created by an older Podman release that is incompatible\n")
		fmt.Fprintf(os.Stderr, "with this release of Podman. It has been started in a limited operational\n")
		fmt.Fprintf(os.Stderr, "mode to allow you to copy any necessary files before recreating it. This\n")
		fmt.Fprintf(os.Stderr, "can be accomplished with the following commands:\n\n")
		fmt.Fprintf(os.Stderr, "\t# Login and copy desired files (Optional)\n")
		fmt.Fprintf(os.Stderr, "\t# Podman machine ssh%s tar cvPf - /path/to/files > backup.tar\n\n", suffix)
		fmt.Fprintf(os.Stderr, "\t# Recreate machine (DESTRUCTIVE!) \n")
		fmt.Fprintf(os.Stderr, "\tpodman machine stop%s\n", suffix)
		fmt.Fprintf(os.Stderr, "\tpodman machine rm -f%s\n", suffix)
		fmt.Fprintf(os.Stderr, "\tpodman machine init --now%s\n\n", suffix)
		fmt.Fprintf(os.Stderr, "\t# Copy back files (Optional)\n")
		fmt.Fprintf(os.Stderr, "\t# cat backup.tar | podman machine ssh%s tar xvPf - \n\n", suffix)
	}

	if forwardState == machine.NoForwarding {
		return
	}

	machine.WaitAndPingAPI(forwardSock)

	if !noInfo {
		if !m.Rootful {
			fmt.Printf("\nThis machine is currently configured in rootless mode. If your containers\n")
			fmt.Printf("require root permissions (e.g. ports < 1024), or if you run into compatibility\n")
			fmt.Printf("issues with non-Podman clients, you can switch using the following command: \n")
			fmt.Printf("\n\tpodman machine set --rootful%s\n\n", suffix)
		}

		fmt.Printf("API forwarding listening on: %s\n", forwardSock)
		if forwardState == machine.DockerGlobal {
			fmt.Printf("Docker API clients default to this address. You do not need to set DOCKER_HOST.\n\n")
		} else {
			stillString := "still "
			switch forwardState {
			case machine.NotInstalled:
				fmt.Printf("\nThe system helper service is not installed; the default Docker API socket\n")
				fmt.Printf("address can't be used by Podman. ")
				if helper := findClaimHelper(); len(helper) > 0 {
					fmt.Printf("If you would like to install it run the\nfollowing commands:\n")
					fmt.Printf("\n\tsudo %s install\n", helper)
					fmt.Printf("\tpodman machine stop%s; podman machine start%s\n\n", suffix, suffix)
				}
			case machine.MachineLocal:
				fmt.Printf("\nAnother process was listening on the default Docker API socket address.\n")
			case machine.ClaimUnsupported:
				fallthrough
			default:
				stillString = ""
			}

			fmt.Printf("You can %sconnect Docker API clients by setting DOCKER_HOST using the\n", stillString)
			fmt.Printf("following command in your terminal session:\n")
			fmt.Printf("\n\texport DOCKER_HOST='unix://%s'\n\n", forwardSock)
		}
	}
}

func (m *MacMachine) isIncompatible() bool {
	return m.UID == -1
}

func generateSystemDFilesForVirtiofsMounts(mounts []machine.VirtIoFs) []machine.Unit {
	var unitFiles []machine.Unit

	for _, mnt := range mounts {
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
WantedBy=multi-user.target`

		virtiofsAutomount := machine.Unit{
			Enabled:  machine.BoolToPtr(true),
			Name:     fmt.Sprintf("%s.automount", mnt.Tag),
			Contents: machine.StrToPtr(fmt.Sprintf(autoMountUnit, mnt.Target, mnt.Target)),
		}
		virtiofsMount := machine.Unit{
			Enabled:  machine.BoolToPtr(true),
			Name:     fmt.Sprintf("%s.mount", mnt.Tag),
			Contents: machine.StrToPtr(fmt.Sprintf(mountUnit, mnt.Tag, mnt.Target)),
		}
		unitFiles = append(unitFiles, virtiofsAutomount, virtiofsMount)
	}
	return unitFiles
}
