//go:build (amd64 && !windows) || (arm64 && !windows)
// +build amd64,!windows arm64,!windows

package qemu

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/utils"
	"github.com/containers/storage/pkg/homedir"
	"github.com/digitalocean/go-qemu/qmp"
	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	qemuProvider = &Provider{}
	// vmtype refers to qemu (vs libvirt, krun, etc)
	vmtype = "qemu"
)

func GetQemuProvider() machine.Provider {
	return qemuProvider
}

const (
	VolumeTypeVirtfs     = "virtfs"
	MountType9p          = "9p"
	dockerSock           = "/var/run/docker.sock"
	dockerConnectTimeout = 5 * time.Second
	apiUpTimeout         = 20 * time.Second
)

type apiForwardingState int

const (
	noForwarding apiForwardingState = iota
	claimUnsupported
	notInstalled
	machineLocal
	dockerGlobal
)

// NewMachine initializes an instance of a virtual machine based on the qemu
// virtualization.
func (p *Provider) NewMachine(opts machine.InitOptions) (machine.VM, error) {
	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return nil, err
	}
	vm := new(MachineVM)
	if len(opts.Name) > 0 {
		vm.Name = opts.Name
	}
	ignitionFile := filepath.Join(vmConfigDir, vm.Name+".ign")
	vm.IgnitionFilePath = ignitionFile

	vm.ImagePath = opts.ImagePath
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

	// Find the qemu executable
	cfg, err := config.Default()
	if err != nil {
		return nil, err
	}
	execPath, err := cfg.FindHelperBinary(QemuCommand, true)
	if err != nil {
		return nil, err
	}

	cmd := []string{execPath}
	// Add memory
	cmd = append(cmd, []string{"-m", strconv.Itoa(int(vm.Memory))}...)
	// Add cpus
	cmd = append(cmd, []string{"-smp", strconv.Itoa(int(vm.CPUs))}...)
	// Add ignition file
	cmd = append(cmd, []string{"-fw_cfg", "name=opt/com.coreos/config,file=" + vm.IgnitionFilePath}...)
	// Add qmp socket
	monitor, err := NewQMPMonitor("unix", vm.Name, defaultQMPTimeout)
	if err != nil {
		return nil, err
	}
	vm.QMPMonitor = monitor
	cmd = append(cmd, []string{"-qmp", monitor.Network + ":/" + monitor.Address + ",server=on,wait=off"}...)

	// Add network
	// Right now the mac address is hardcoded so that the host networking gives it a specific IP address.  This is
	// why we can only run one vm at a time right now
	cmd = append(cmd, []string{"-netdev", "socket,id=vlan,fd=3", "-device", "virtio-net-pci,netdev=vlan,mac=5a:94:ef:e4:0c:ee"}...)
	socketPath, err := getRuntimeDir()
	if err != nil {
		return nil, err
	}
	virtualSocketPath := filepath.Join(socketPath, "podman", vm.Name+"_ready.sock")
	// Add serial port for readiness
	cmd = append(cmd, []string{
		"-device", "virtio-serial",
		"-chardev", "socket,path=" + virtualSocketPath + ",server=on,wait=off,id=" + vm.Name + "_ready",
		"-device", "virtserialport,chardev=" + vm.Name + "_ready" + ",name=org.fedoraproject.port.0"}...)
	vm.CmdLine = cmd
	return vm, nil
}

// LoadByName reads a json file that describes a known qemu vm
// and returns a vm instance
func (p *Provider) LoadVMByName(name string) (machine.VM, error) {
	vm := &MachineVM{UID: -1} // posix reserves -1, so use it to signify undefined
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
	err = json.Unmarshal(b, vm)

	// It is here for providing the ability to propagate
	// proxy settings (e.g. HTTP_PROXY and others) on a start
	// and avoid a need of re-creating/re-initiating a VM
	if proxyOpts := machine.GetProxyVariables(); len(proxyOpts) > 0 {
		proxyStr := "name=opt/com.coreos/environment,string="
		var proxies string
		for k, v := range proxyOpts {
			proxies = fmt.Sprintf("%s%s=\"%s\"|", proxies, k, v)
		}
		proxyStr = fmt.Sprintf("%s%s", proxyStr, base64.StdEncoding.EncodeToString([]byte(proxies)))
		vm.CmdLine = append(vm.CmdLine, "-fw_cfg", proxyStr)
	}

	logrus.Debug(vm.CmdLine)
	return vm, err
}

// Init writes the json configuration file to the filesystem for
// other verbs (start, stop)
func (v *MachineVM) Init(opts machine.InitOptions) (bool, error) {
	var (
		key string
	)
	sshDir := filepath.Join(homedir.Get(), ".ssh")
	v.IdentityPath = filepath.Join(sshDir, v.Name)
	v.Rootful = opts.Rootful

	switch opts.ImagePath {
	case "testing", "next", "stable", "":
		// Get image as usual
		v.ImageStream = opts.ImagePath
		dd, err := machine.NewFcosDownloader(vmtype, v.Name, opts.ImagePath)

		if err != nil {
			return false, err
		}
		v.ImagePath = dd.Get().LocalUncompressedFile
		if err := machine.DownloadImage(dd); err != nil {
			return false, err
		}
	default:
		// The user has provided an alternate image which can be a file path
		// or URL.
		v.ImageStream = "custom"
		g, err := machine.NewGenericDownloader(vmtype, v.Name, opts.ImagePath)
		if err != nil {
			return false, err
		}
		v.ImagePath = g.Get().LocalUncompressedFile
		if err := machine.DownloadImage(g); err != nil {
			return false, err
		}
	}
	// Add arch specific options including image location
	v.CmdLine = append(v.CmdLine, v.addArchOptions()...)

	var volumeType string
	switch opts.VolumeDriver {
	case "virtfs":
		volumeType = VolumeTypeVirtfs
	case "": // default driver
		volumeType = VolumeTypeVirtfs
	default:
		err := fmt.Errorf("unknown volume driver: %s", opts.VolumeDriver)
		return false, err
	}

	mounts := []Mount{}
	for i, volume := range opts.Volumes {
		tag := fmt.Sprintf("vol%d", i)
		paths := strings.SplitN(volume, ":", 3)
		source := paths[0]
		target := source
		readonly := false
		if len(paths) > 1 {
			target = paths[1]
		}
		if len(paths) > 2 {
			options := paths[2]
			volopts := strings.Split(options, ",")
			for _, o := range volopts {
				switch o {
				case "rw":
					readonly = false
				case "ro":
					readonly = true
				default:
					fmt.Printf("Unknown option: %s\n", o)
				}
			}
		}
		switch volumeType {
		case VolumeTypeVirtfs:
			virtfsOptions := fmt.Sprintf("local,path=%s,mount_tag=%s,security_model=mapped-xattr", source, tag)
			if readonly {
				virtfsOptions += ",readonly"
			}
			v.CmdLine = append(v.CmdLine, []string{"-virtfs", virtfsOptions}...)
			mounts = append(mounts, Mount{Type: MountType9p, Tag: tag, Source: source, Target: target, ReadOnly: readonly})
		}
	}
	v.Mounts = mounts
	v.UID = os.Getuid()

	// Add location of bootable image
	v.CmdLine = append(v.CmdLine, "-drive", "if=virtio,file="+v.ImagePath)
	// This kind of stinks but no other way around this r/n
	if len(opts.IgnitionPath) < 1 {
		uri := machine.SSHRemoteConnection.MakeSSHURL("localhost", fmt.Sprintf("/run/user/%d/podman/podman.sock", v.UID), strconv.Itoa(v.Port), v.RemoteUsername)
		uriRoot := machine.SSHRemoteConnection.MakeSSHURL("localhost", "/run/podman/podman.sock", strconv.Itoa(v.Port), "root")
		identity := filepath.Join(sshDir, v.Name)

		uris := []url.URL{uri, uriRoot}
		names := []string{v.Name, v.Name + "-root"}

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
	// Write the JSON file
	v.writeConfig()

	// User has provided ignition file so keygen
	// will be skipped.
	if len(opts.IgnitionPath) < 1 {
		var err error
		key, err = machine.CreateSSHKeys(v.IdentityPath)
		if err != nil {
			return false, err
		}
	}
	// Run arch specific things that need to be done
	if err := v.prepare(); err != nil {
		return false, err
	}

	originalDiskSize, err := getDiskSize(v.ImagePath)
	if err != nil {
		return false, err
	}
	// Resize the disk image to input disk size
	// only if the virtualdisk size is less than
	// the given disk size
	if opts.DiskSize<<(10*3) > originalDiskSize {
		// Find the qemu executable
		cfg, err := config.Default()
		if err != nil {
			return false, err
		}
		resizePath, err := cfg.FindHelperBinary("qemu-img", true)
		if err != nil {
			return false, err
		}
		resize := exec.Command(resizePath, []string{"resize", v.ImagePath, strconv.Itoa(int(opts.DiskSize)) + "G"}...)
		resize.Stdout = os.Stdout
		resize.Stderr = os.Stderr
		if err := resize.Run(); err != nil {
			return false, errors.Errorf("error resizing image: %q", err)
		}
	}
	// If the user provides an ignition file, we need to
	// copy it into the conf dir
	if len(opts.IgnitionPath) > 0 {
		inputIgnition, err := ioutil.ReadFile(opts.IgnitionPath)
		if err != nil {
			return false, err
		}
		return false, ioutil.WriteFile(v.IgnitionFilePath, inputIgnition, 0644)
	}
	// Write the ignition file
	ign := machine.DynamicIgnition{
		Name:      opts.Username,
		Key:       key,
		VMName:    v.Name,
		TimeZone:  opts.TimeZone,
		WritePath: v.IgnitionFilePath,
		UID:       v.UID,
	}
	err = machine.NewIgnitionFile(ign)
	return err == nil, err
}

func (v *MachineVM) Set(name string, opts machine.SetOptions) error {
	if v.Rootful == opts.Rootful {
		return nil
	}

	changeCon, err := machine.AnyConnectionDefault(v.Name, v.Name+"-root")
	if err != nil {
		return err
	}

	if changeCon {
		newDefault := v.Name
		if opts.Rootful {
			newDefault += "-root"
		}
		if err := machine.ChangeDefault(newDefault); err != nil {
			return err
		}
	}

	v.Rootful = opts.Rootful
	return v.writeConfig()
}

// Start executes the qemu command line and forks it
func (v *MachineVM) Start(name string, _ machine.StartOptions) error {
	var (
		conn           net.Conn
		err            error
		qemuSocketConn net.Conn
		wait           time.Duration = time.Millisecond * 500
	)

	if v.isIncompatible() {
		logrus.Errorf("machine %q is incompatible with this release of podman and needs to be recreated, starting for recovery only", v.Name)
	}

	forwardSock, forwardState, err := v.startHostNetworking()
	if err != nil {
		return errors.Errorf("unable to start host networking: %q", err)
	}

	rtPath, err := getRuntimeDir()
	if err != nil {
		return err
	}

	// If the temporary podman dir is not created, create it
	podmanTempDir := filepath.Join(rtPath, "podman")
	if _, err := os.Stat(podmanTempDir); os.IsNotExist(err) {
		if mkdirErr := os.MkdirAll(podmanTempDir, 0755); mkdirErr != nil {
			return err
		}
	}
	qemuSocketPath, _, err := v.getSocketandPid()
	if err != nil {
		return err
	}
	// If the qemusocketpath exists and the vm is off/down, we should rm
	// it before the dial as to avoid a segv
	if err := os.Remove(qemuSocketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		logrus.Warn(err)
	}
	for i := 0; i < 6; i++ {
		qemuSocketConn, err = net.Dial("unix", qemuSocketPath)
		if err == nil {
			break
		}
		time.Sleep(wait)
		wait++
	}
	if err != nil {
		return err
	}

	fd, err := qemuSocketConn.(*net.UnixConn).File()
	if err != nil {
		return err
	}

	attr := new(os.ProcAttr)
	files := []*os.File{os.Stdin, os.Stdout, os.Stderr, fd}
	attr.Files = files
	logrus.Debug(v.CmdLine)
	cmd := v.CmdLine

	// Disable graphic window when not in debug mode
	// Done in start, so we're not suck with the debug level we used on init
	if !logrus.IsLevelEnabled(logrus.DebugLevel) {
		cmd = append(cmd, "-display", "none")
	}

	_, err = os.StartProcess(v.CmdLine[0], cmd, attr)
	if err != nil {
		// check if qemu was not found
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		// lookup qemu again maybe the path was changed, https://github.com/containers/podman/issues/13394
		cfg, err := config.Default()
		if err != nil {
			return err
		}
		cmd[0], err = cfg.FindHelperBinary(QemuCommand, true)
		if err != nil {
			return err
		}
		_, err = os.StartProcess(cmd[0], cmd, attr)
		if err != nil {
			return err
		}
	}
	fmt.Println("Waiting for VM ...")
	socketPath, err := getRuntimeDir()
	if err != nil {
		return err
	}

	// The socket is not made until the qemu process is running so here
	// we do a backoff waiting for it.  Once we have a conn, we break and
	// then wait to read it.
	for i := 0; i < 6; i++ {
		conn, err = net.Dial("unix", filepath.Join(socketPath, "podman", v.Name+"_ready.sock"))
		if err == nil {
			break
		}
		time.Sleep(wait)
		wait++
	}
	if err != nil {
		return err
	}
	_, err = bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return err
	}

	if len(v.Mounts) > 0 {
		running, err := v.isRunning()
		if err != nil {
			return err
		}
		listening := v.isListening()
		for !running || !listening {
			time.Sleep(100 * time.Millisecond)
			running, err = v.isRunning()
			if err != nil {
				return err
			}
			listening = v.isListening()
		}
	}
	for _, mount := range v.Mounts {
		fmt.Printf("Mounting volume... %s:%s\n", mount.Source, mount.Target)
		// create mountpoint directory if it doesn't exist
		// because / is immutable, we have to monkey around with permissions
		// if we dont mount in /home or /mnt
		args := []string{"-q", "--"}
		if !strings.HasPrefix(mount.Target, "/home") || !strings.HasPrefix(mount.Target, "/mnt") {
			args = append(args, "sudo", "chattr", "-i", "/", ";")
		}
		args = append(args, "sudo", "mkdir", "-p", mount.Target)
		if !strings.HasPrefix(mount.Target, "/home") || !strings.HasPrefix(mount.Target, "/mnt") {
			args = append(args, ";", "sudo", "chattr", "+i", "/", ";")
		}
		err = v.SSH(name, machine.SSHOptions{Args: args})
		if err != nil {
			return err
		}
		switch mount.Type {
		case MountType9p:
			mountOptions := []string{"-t", "9p"}
			mountOptions = append(mountOptions, []string{"-o", "trans=virtio", mount.Tag, mount.Target}...)
			mountOptions = append(mountOptions, []string{"-o", "version=9p2000.L,msize=131072"}...)
			if mount.ReadOnly {
				mountOptions = append(mountOptions, []string{"-o", "ro"}...)
			}
			err = v.SSH(name, machine.SSHOptions{Args: append([]string{"-q", "--", "sudo", "mount"}, mountOptions...)})
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown mount type: %s", mount.Type)
		}
	}

	v.waitAPIAndPrintInfo(forwardState, forwardSock)

	return nil
}

func (v *MachineVM) checkStatus(monitor *qmp.SocketMonitor) (machine.QemuMachineStatus, error) {
	// this is the format returned from the monitor
	// {"return": {"status": "running", "singlestep": false, "running": true}}

	type statusDetails struct {
		Status  string `json:"status"`
		Step    bool   `json:"singlestep"`
		Running bool   `json:"running"`
	}
	type statusResponse struct {
		Response statusDetails `json:"return"`
	}
	var response statusResponse

	checkCommand := struct {
		Execute string `json:"execute"`
	}{
		Execute: "query-status",
	}
	input, err := json.Marshal(checkCommand)
	if err != nil {
		return "", err
	}
	b, err := monitor.Run(input)
	if err != nil {
		if errors.Cause(err) == os.ErrNotExist {
			return machine.Stopped, nil
		}
		return "", err
	}
	if err := json.Unmarshal(b, &response); err != nil {
		return "", err
	}
	if response.Response.Status == machine.Running {
		return machine.Running, nil
	}
	return machine.Stopped, nil
}

// Stop uses the qmp monitor to call a system_powerdown
func (v *MachineVM) Stop(name string, _ machine.StopOptions) error {
	var disconnected bool
	// check if the qmp socket is there. if not, qemu instance is gone
	if _, err := os.Stat(v.QMPMonitor.Address); os.IsNotExist(err) {
		// Right now it is NOT an error to stop a stopped machine
		logrus.Debugf("QMP monitor socket %v does not exist", v.QMPMonitor.Address)
		return nil
	}
	qmpMonitor, err := qmp.NewSocketMonitor(v.QMPMonitor.Network, v.QMPMonitor.Address, v.QMPMonitor.Timeout)
	if err != nil {
		return err
	}
	// Simple JSON formation for the QAPI
	stopCommand := struct {
		Execute string `json:"execute"`
	}{
		Execute: "system_powerdown",
	}
	input, err := json.Marshal(stopCommand)
	if err != nil {
		return err
	}
	if err := qmpMonitor.Connect(); err != nil {
		return err
	}
	defer func() {
		if !disconnected {
			if err := qmpMonitor.Disconnect(); err != nil {
				logrus.Error(err)
			}
		}
	}()

	if _, err = qmpMonitor.Run(input); err != nil {
		return err
	}

	qemuSocketFile, pidFile, err := v.getSocketandPid()
	if err != nil {
		return err
	}
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		return nil
	}
	pidString, err := ioutil.ReadFile(pidFile)
	if err != nil {
		return err
	}
	pidNum, err := strconv.Atoi(string(pidString))
	if err != nil {
		return err
	}

	p, err := os.FindProcess(pidNum)
	if p == nil && err != nil {
		return err
	}
	// Kill the process
	if err := p.Kill(); err != nil {
		return err
	}
	// Remove the pidfile
	if err := os.Remove(pidFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		logrus.Warn(err)
	}
	// Remove socket
	if err := os.Remove(qemuSocketFile); err != nil {
		return err
	}

	if err := qmpMonitor.Disconnect(); err != nil {
		return nil
	}

	disconnected = true
	waitInternal := 250 * time.Millisecond
	for i := 0; i < 5; i++ {
		running, err := v.isRunning()
		if err != nil {
			return err
		}
		if !running {
			break
		}
		time.Sleep(waitInternal)
		waitInternal = waitInternal * 2
	}

	return nil
}

// NewQMPMonitor creates the monitor subsection of our vm
func NewQMPMonitor(network, name string, timeout time.Duration) (Monitor, error) {
	rtDir, err := getRuntimeDir()
	if err != nil {
		return Monitor{}, err
	}
	if !rootless.IsRootless() {
		rtDir = "/run"
	}
	rtDir = filepath.Join(rtDir, "podman")
	if _, err := os.Stat(filepath.Join(rtDir)); os.IsNotExist(err) {
		// TODO 0644 is fine on linux but macos is weird
		if err := os.MkdirAll(rtDir, 0755); err != nil {
			return Monitor{}, err
		}
	}
	if timeout == 0 {
		timeout = defaultQMPTimeout
	}
	monitor := Monitor{
		Network: network,
		Address: filepath.Join(rtDir, "qmp_"+name+".sock"),
		Timeout: timeout,
	}
	return monitor, nil
}

func (v *MachineVM) Remove(name string, opts machine.RemoveOptions) (string, func() error, error) {
	var (
		files []string
	)

	// cannot remove a running vm
	running, err := v.isRunning()
	if err != nil {
		return "", nil, err
	}
	if running && !opts.Force {
		return "", nil, errors.Errorf("running vm %q cannot be destroyed", v.Name)
	}

	// Collect all the files that need to be destroyed
	if !opts.SaveKeys {
		files = append(files, v.IdentityPath, v.IdentityPath+".pub")
	}
	if !opts.SaveIgnition {
		files = append(files, v.IgnitionFilePath)
	}
	if !opts.SaveImage {
		files = append(files, v.ImagePath)
	}
	socketPath, err := v.getForwardSocketPath()
	if err != nil {
		logrus.Error(err)
	}
	files = append(files, socketPath)
	files = append(files, v.archRemovalFiles()...)

	if err := machine.RemoveConnection(v.Name); err != nil {
		logrus.Error(err)
	}
	if err := machine.RemoveConnection(v.Name + "-root"); err != nil {
		logrus.Error(err)
	}

	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return "", nil, err
	}
	files = append(files, filepath.Join(vmConfigDir, v.Name+".json"))
	confirmationMessage := "\nThe following files will be deleted:\n\n"
	for _, msg := range files {
		confirmationMessage += msg + "\n"
	}

	// Get path to socket and pidFile before we do any cleanups
	qemuSocketFile, pidFile, errSocketFile := v.getSocketandPid()
	//silently try to delete socket and pid file
	//remove socket and pid file if any: warn at low priority if things fail
	if errSocketFile == nil {
		// Remove the pidfile
		if err := os.Remove(pidFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			logrus.Debugf("Error while removing pidfile: %v", err)
		}
		// Remove socket
		if err := os.Remove(qemuSocketFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			logrus.Debugf("Error while removing podman-machine-socket: %v", err)
		}
	}

	confirmationMessage += "\n"
	return confirmationMessage, func() error {
		for _, f := range files {
			if err := os.Remove(f); err != nil {
				logrus.Error(err)
			}
		}
		return nil
	}, nil
}

func (v *MachineVM) isRunning() (bool, error) {
	// Check if qmp socket path exists
	if _, err := os.Stat(v.QMPMonitor.Address); os.IsNotExist(err) {
		return false, nil
	}
	// Check if we can dial it
	monitor, err := qmp.NewSocketMonitor(v.QMPMonitor.Network, v.QMPMonitor.Address, v.QMPMonitor.Timeout)
	if err != nil {
		return false, nil
	}
	if err := monitor.Connect(); err != nil {
		return false, err
	}
	defer func() {
		if err := monitor.Disconnect(); err != nil {
			logrus.Error(err)
		}
	}()
	// If there is a monitor, lets see if we can query state
	state, err := v.checkStatus(monitor)
	if err != nil {
		return false, err
	}
	if state == machine.Running {
		return true, nil
	}
	return false, nil
}

func (v *MachineVM) isListening() bool {
	// Check if we can dial it
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", "127.0.0.1", v.Port), 10*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// SSH opens an interactive SSH session to the vm specified.
// Added ssh function to VM interface: pkg/machine/config/go : line 58
func (v *MachineVM) SSH(name string, opts machine.SSHOptions) error {
	running, err := v.isRunning()
	if err != nil {
		return err
	}
	if !running {
		return errors.Errorf("vm %q is not running.", v.Name)
	}

	username := opts.Username
	if username == "" {
		username = v.RemoteUsername
	}

	sshDestination := username + "@localhost"
	port := strconv.Itoa(v.Port)

	args := []string{"-i", v.IdentityPath, "-p", port, sshDestination, "-o", "UserKnownHostsFile /dev/null", "-o", "StrictHostKeyChecking no"}
	if len(opts.Args) > 0 {
		args = append(args, opts.Args...)
	} else {
		fmt.Printf("Connecting to vm %s. To close connection, use `~.` or `exit`\n", v.Name)
	}

	cmd := exec.Command("ssh", args...)
	logrus.Debugf("Executing: ssh %v\n", args)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// executes qemu-image info to get the virtual disk size
// of the diskimage
func getDiskSize(path string) (uint64, error) {
	// Find the qemu executable
	cfg, err := config.Default()
	if err != nil {
		return 0, err
	}
	qemuPathDir, err := cfg.FindHelperBinary("qemu-img", true)
	if err != nil {
		return 0, err
	}
	diskInfo := exec.Command(qemuPathDir, "info", "--output", "json", path)
	stdout, err := diskInfo.StdoutPipe()
	if err != nil {
		return 0, err
	}
	if err := diskInfo.Start(); err != nil {
		return 0, err
	}
	tmpInfo := struct {
		VirtualSize    uint64 `json:"virtual-size"`
		Filename       string `json:"filename"`
		ClusterSize    int64  `json:"cluster-size"`
		Format         string `json:"format"`
		FormatSpecific struct {
			Type string            `json:"type"`
			Data map[string]string `json:"data"`
		}
		DirtyFlag bool `json:"dirty-flag"`
	}{}
	if err := json.NewDecoder(stdout).Decode(&tmpInfo); err != nil {
		return 0, err
	}
	if err := diskInfo.Wait(); err != nil {
		return 0, err
	}
	return tmpInfo.VirtualSize, nil
}

// List lists all vm's that use qemu virtualization
func (p *Provider) List(_ machine.ListOptions) ([]*machine.ListResponse, error) {
	return GetVMInfos()
}

func GetVMInfos() ([]*machine.ListResponse, error) {
	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return nil, err
	}

	var listed []*machine.ListResponse

	if err = filepath.WalkDir(vmConfigDir, func(path string, d fs.DirEntry, err error) error {
		vm := new(MachineVM)
		if strings.HasSuffix(d.Name(), ".json") {
			fullPath := filepath.Join(vmConfigDir, d.Name())
			b, err := ioutil.ReadFile(fullPath)
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
			listEntry.VMType = "qemu"
			listEntry.CPUs = vm.CPUs
			listEntry.Memory = vm.Memory * units.MiB
			listEntry.DiskSize = vm.DiskSize * units.GiB
			listEntry.Port = vm.Port
			listEntry.RemoteUsername = vm.RemoteUsername
			listEntry.IdentityPath = vm.IdentityPath
			fi, err := os.Stat(fullPath)
			if err != nil {
				return err
			}
			listEntry.CreatedAt = fi.ModTime()

			fi, err = os.Stat(vm.ImagePath)
			if err != nil {
				return err
			}
			listEntry.LastUp = fi.ModTime()
			running, err := vm.isRunning()
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

func (p *Provider) IsValidVMName(name string) (bool, error) {
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

// CheckExclusiveActiveVM checks if there is a VM already running
// that does not allow other VMs to be running
func (p *Provider) CheckExclusiveActiveVM() (bool, string, error) {
	vms, err := GetVMInfos()
	if err != nil {
		return false, "", errors.Wrap(err, "error checking VM active")
	}
	for _, vm := range vms {
		if vm.Running {
			return true, vm.Name, nil
		}
	}
	return false, "", nil
}

// startHostNetworking runs a binary on the host system that allows users
// to setup port forwarding to the podman virtual machine
func (v *MachineVM) startHostNetworking() (string, apiForwardingState, error) {
	cfg, err := config.Default()
	if err != nil {
		return "", noForwarding, err
	}
	binary, err := cfg.FindHelperBinary(machine.ForwarderBinaryName, false)
	if err != nil {
		return "", noForwarding, err
	}

	qemuSocket, pidFile, err := v.getSocketandPid()
	if err != nil {
		return "", noForwarding, err
	}
	attr := new(os.ProcAttr)
	// Pass on stdin, stdout, stderr
	files := []*os.File{os.Stdin, os.Stdout, os.Stderr}
	attr.Files = files
	cmd := []string{binary}
	cmd = append(cmd, []string{"-listen-qemu", fmt.Sprintf("unix://%s", qemuSocket), "-pid-file", pidFile}...)
	// Add the ssh port
	cmd = append(cmd, []string{"-ssh-port", fmt.Sprintf("%d", v.Port)}...)

	var forwardSock string
	var state apiForwardingState
	if !v.isIncompatible() {
		cmd, forwardSock, state = v.setupAPIForwarding(cmd)
	}

	if logrus.GetLevel() == logrus.DebugLevel {
		cmd = append(cmd, "--debug")
		fmt.Println(cmd)
	}
	_, err = os.StartProcess(cmd[0], cmd, attr)
	return forwardSock, state, err
}

func (v *MachineVM) setupAPIForwarding(cmd []string) ([]string, string, apiForwardingState) {
	socket, err := v.getForwardSocketPath()

	if err != nil {
		return cmd, "", noForwarding
	}

	destSock := fmt.Sprintf("/run/user/%d/podman/podman.sock", v.UID)
	forwardUser := "core"

	if v.Rootful {
		destSock = "/run/podman/podman.sock"
		forwardUser = "root"
	}

	cmd = append(cmd, []string{"-forward-sock", socket}...)
	cmd = append(cmd, []string{"-forward-dest", destSock}...)
	cmd = append(cmd, []string{"-forward-user", forwardUser}...)
	cmd = append(cmd, []string{"-forward-identity", v.IdentityPath}...)
	link := filepath.Join(filepath.Dir(filepath.Dir(socket)), "podman.sock")

	// The linking pattern is /var/run/docker.sock -> user global sock (link) -> machine sock (socket)
	// This allows the helper to only have to maintain one constant target to the user, which can be
	// repositioned without updating docker.sock.
	if !dockerClaimSupported() {
		return cmd, socket, claimUnsupported
	}

	if !dockerClaimHelperInstalled() {
		return cmd, socket, notInstalled
	}

	if !alreadyLinked(socket, link) {
		if checkSockInUse(link) {
			return cmd, socket, machineLocal
		}

		_ = os.Remove(link)
		if err = os.Symlink(socket, link); err != nil {
			logrus.Warnf("could not create user global API forwarding link: %s", err.Error())
			return cmd, socket, machineLocal
		}
	}

	if !alreadyLinked(link, dockerSock) {
		if checkSockInUse(dockerSock) {
			return cmd, socket, machineLocal
		}

		if !claimDockerSock() {
			logrus.Warn("podman helper is installed, but was not able to claim the global docker sock")
			return cmd, socket, machineLocal
		}
	}

	return cmd, dockerSock, dockerGlobal
}

func (v *MachineVM) isIncompatible() bool {
	return v.UID == -1
}

func (v *MachineVM) getForwardSocketPath() (string, error) {
	path, err := machine.GetDataDir(v.Name)
	if err != nil {
		logrus.Errorf("Error resolving data dir: %s", err.Error())
		return "", nil
	}
	return filepath.Join(path, "podman.sock"), nil
}

func (v *MachineVM) getSocketandPid() (string, string, error) {
	rtPath, err := getRuntimeDir()
	if err != nil {
		return "", "", err
	}
	if !rootless.IsRootless() {
		rtPath = "/run"
	}
	socketDir := filepath.Join(rtPath, "podman")
	pidFile := filepath.Join(socketDir, fmt.Sprintf("%s.pid", v.Name))
	qemuSocket := filepath.Join(socketDir, fmt.Sprintf("qemu_%s.sock", v.Name))
	return qemuSocket, pidFile, nil
}

func checkSockInUse(sock string) bool {
	if info, err := os.Stat(sock); err == nil && info.Mode()&fs.ModeSocket == fs.ModeSocket {
		_, err = net.DialTimeout("unix", dockerSock, dockerConnectTimeout)
		return err == nil
	}

	return false
}

func alreadyLinked(target string, link string) bool {
	read, err := os.Readlink(link)
	return err == nil && read == target
}

func waitAndPingAPI(sock string) {
	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(context.Context, string, string) (net.Conn, error) {
				con, err := net.DialTimeout("unix", sock, apiUpTimeout)
				if err == nil {
					con.SetDeadline(time.Now().Add(apiUpTimeout))
				}
				return con, err
			},
		},
	}

	resp, err := client.Get("http://host/_ping")
	if err == nil {
		defer resp.Body.Close()
	}
	if err != nil || resp.StatusCode != 200 {
		logrus.Warn("API socket failed ping test")
	}
}

func (v *MachineVM) waitAPIAndPrintInfo(forwardState apiForwardingState, forwardSock string) {
	suffix := ""
	if v.Name != machine.DefaultMachineName {
		suffix = " " + v.Name
	}

	if v.isIncompatible() {
		fmt.Fprintf(os.Stderr, "\n!!! ACTION REQUIRED: INCOMPATIBLE MACHINE !!!\n")

		fmt.Fprintf(os.Stderr, "\nThis machine was created by an older podman release that is incompatible\n")
		fmt.Fprintf(os.Stderr, "with this release of podman. It has been started in a limited operational\n")
		fmt.Fprintf(os.Stderr, "mode to allow you to copy any necessary files before recreating it. This\n")
		fmt.Fprintf(os.Stderr, "can be accomplished with the following commands:\n\n")
		fmt.Fprintf(os.Stderr, "\t# Login and copy desired files (Optional)\n")
		fmt.Fprintf(os.Stderr, "\t# podman machine ssh%s tar cvPf - /path/to/files > backup.tar\n\n", suffix)
		fmt.Fprintf(os.Stderr, "\t# Recreate machine (DESTRUCTIVE!) \n")
		fmt.Fprintf(os.Stderr, "\tpodman machine stop%s\n", suffix)
		fmt.Fprintf(os.Stderr, "\tpodman machine rm -f%s\n", suffix)
		fmt.Fprintf(os.Stderr, "\tpodman machine init --now%s\n\n", suffix)
		fmt.Fprintf(os.Stderr, "\t# Copy back files (Optional)\n")
		fmt.Fprintf(os.Stderr, "\t# cat backup.tar | podman machine ssh%s tar xvPf - \n\n", suffix)
	}

	if forwardState == noForwarding {
		return
	}

	waitAndPingAPI(forwardSock)
	if !v.Rootful {
		fmt.Printf("\nThis machine is currently configured in rootless mode. If your containers\n")
		fmt.Printf("require root permissions (e.g. ports < 1024), or if you run into compatibility\n")
		fmt.Printf("issues with non-podman clients, you can switch using the following command: \n")
		fmt.Printf("\n\tpodman machine set --rootful%s\n\n", suffix)
	}

	fmt.Printf("API forwarding listening on: %s\n", forwardSock)
	if forwardState == dockerGlobal {
		fmt.Printf("Docker API clients default to this address. You do not need to set DOCKER_HOST.\n\n")
	} else {
		stillString := "still "
		switch forwardState {
		case notInstalled:
			fmt.Printf("\nThe system helper service is not installed; the default Docker API socket\n")
			fmt.Printf("address can't be used by podman. ")
			if helper := findClaimHelper(); len(helper) > 0 {
				fmt.Printf("If you would like to install it run the\nfollowing commands:\n")
				fmt.Printf("\n\tsudo %s install\n", helper)
				fmt.Printf("\tpodman machine stop%s; podman machine start%s\n\n", suffix, suffix)
			}
		case machineLocal:
			fmt.Printf("\nAnother process was listening on the default Docker API socket address.\n")
		case claimUnsupported:
			fallthrough
		default:
			stillString = ""
		}

		fmt.Printf("You can %sconnect Docker API clients by setting DOCKER_HOST using the\n", stillString)
		fmt.Printf("following command in your terminal session:\n")
		fmt.Printf("\n\texport DOCKER_HOST='unix://%s'\n\n", forwardSock)
	}
}

func (v *MachineVM) writeConfig() error {
	// GetConfDir creates the directory so no need to check for
	// its existence
	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return err
	}

	jsonFile := filepath.Join(vmConfigDir, v.Name) + ".json"
	// Write the JSON file
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(jsonFile, b, 0644); err != nil {
		return err
	}

	return nil
}
