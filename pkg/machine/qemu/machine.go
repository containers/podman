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
	// vmtype refers to qemu (vs libvirt, krun, etc).
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
	ignitionFile, err := machine.NewMachineFile(filepath.Join(vmConfigDir, vm.Name+".ign"), nil)
	if err != nil {
		return nil, err
	}
	vm.IgnitionFile = *ignitionFile
	imagePath, err := machine.NewMachineFile(opts.ImagePath, nil)
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

	vm.Created = time.Now()

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
	cmd = append(cmd, []string{"-fw_cfg", "name=opt/com.coreos/config,file=" + vm.IgnitionFile.GetPath()}...)
	// Add qmp socket
	monitor, err := NewQMPMonitor("unix", vm.Name, defaultQMPTimeout)
	if err != nil {
		return nil, err
	}
	vm.QMPMonitor = monitor
	cmd = append(cmd, []string{"-qmp", monitor.Network + ":/" + monitor.Address.GetPath() + ",server=on,wait=off"}...)

	// Add network
	// Right now the mac address is hardcoded so that the host networking gives it a specific IP address.  This is
	// why we can only run one vm at a time right now
	cmd = append(cmd, []string{"-netdev", "socket,id=vlan,fd=3", "-device", "virtio-net-pci,netdev=vlan,mac=5a:94:ef:e4:0c:ee"}...)
	if err := vm.setReadySocket(); err != nil {
		return nil, err
	}

	// Add serial port for readiness
	cmd = append(cmd, []string{
		"-device", "virtio-serial",
		// qemu needs to establish the long name; other connections can use the symlink'd
		"-chardev", "socket,path=" + vm.ReadySocket.Path + ",server=on,wait=off,id=" + vm.Name + "_ready",
		"-device", "virtserialport,chardev=" + vm.Name + "_ready" + ",name=org.fedoraproject.port.0"}...)
	vm.CmdLine = cmd
	if err := vm.setPIDSocket(); err != nil {
		return nil, err
	}
	return vm, nil
}

// migrateVM takes the old configuration structure and migrates it
// to the new structure and writes it to the filesystem
func migrateVM(configPath string, config []byte, vm *MachineVM) error {
	fmt.Printf("Migrating machine %q\n", vm.Name)
	var old MachineVMV1
	err := json.Unmarshal(config, &old)
	if err != nil {
		return err
	}
	// Looks like we loaded the older structure; now we need to migrate
	// from the old structure to the new structure
	_, pidFile, err := vm.getSocketandPid()
	if err != nil {
		return err
	}

	pidFilePath := machine.VMFile{Path: pidFile}
	qmpMonitor := Monitor{
		Address: machine.VMFile{Path: old.QMPMonitor.Address},
		Network: old.QMPMonitor.Network,
		Timeout: old.QMPMonitor.Timeout,
	}
	socketPath, err := getRuntimeDir()
	if err != nil {
		return err
	}
	virtualSocketPath := filepath.Join(socketPath, "podman", vm.Name+"_ready.sock")
	readySocket := machine.VMFile{Path: virtualSocketPath}

	vm.HostUser = machine.HostUser{}
	vm.ImageConfig = machine.ImageConfig{}
	vm.ResourceConfig = machine.ResourceConfig{}
	vm.SSHConfig = machine.SSHConfig{}

	ignitionFilePath, err := machine.NewMachineFile(old.IgnitionFilePath, nil)
	if err != nil {
		return err
	}
	imagePath, err := machine.NewMachineFile(old.ImagePath, nil)
	if err != nil {
		return err
	}

	// setReadySocket will stick the entry into the new struct
	if err := vm.setReadySocket(); err != nil {
		return err
	}

	vm.CPUs = old.CPUs
	vm.CmdLine = old.CmdLine
	vm.DiskSize = old.DiskSize
	vm.IdentityPath = old.IdentityPath
	vm.IgnitionFile = *ignitionFilePath
	vm.ImagePath = *imagePath
	vm.ImageStream = old.ImageStream
	vm.Memory = old.Memory
	vm.Mounts = old.Mounts
	vm.Name = old.Name
	vm.PidFilePath = pidFilePath
	vm.Port = old.Port
	vm.QMPMonitor = qmpMonitor
	vm.ReadySocket = readySocket
	vm.RemoteUsername = old.RemoteUsername
	vm.Rootful = old.Rootful
	vm.UID = old.UID

	// Backup the original config file
	if err := os.Rename(configPath, configPath+".orig"); err != nil {
		return err
	}
	// Write the config file
	if err := vm.writeConfig(); err != nil {
		// If the config file fails to be written, put the origina
		// config file back before erroring
		if renameError := os.Rename(configPath+".orig", configPath); renameError != nil {
			logrus.Warn(renameError)
		}
		return err
	}
	// Remove the backup file
	return os.Remove(configPath + ".orig")
}

// LoadVMByName reads a json file that describes a known qemu vm
// and returns a vm instance
func (p *Provider) LoadVMByName(name string) (machine.VM, error) {
	vm := &MachineVM{Name: name}
	vm.HostUser = machine.HostUser{UID: -1} // posix reserves -1, so use it to signify undefined
	if err := vm.update(); err != nil {
		return nil, err
	}

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
	return vm, nil
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
	case Testing, Next, Stable, "":
		// Get image as usual
		v.ImageStream = opts.ImagePath
		dd, err := machine.NewFcosDownloader(vmtype, v.Name, opts.ImagePath)

		if err != nil {
			return false, err
		}
		uncompressedFile, err := machine.NewMachineFile(dd.Get().LocalUncompressedFile, nil)
		if err != nil {
			return false, err
		}
		v.ImagePath = *uncompressedFile
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
		imagePath, err := machine.NewMachineFile(g.Get().LocalUncompressedFile, nil)
		if err != nil {
			return false, err
		}
		v.ImagePath = *imagePath
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

	mounts := []machine.Mount{}
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
		if volumeType == VolumeTypeVirtfs {
			virtfsOptions := fmt.Sprintf("local,path=%s,mount_tag=%s,security_model=mapped-xattr", source, tag)
			if readonly {
				virtfsOptions += ",readonly"
			}
			v.CmdLine = append(v.CmdLine, []string{"-virtfs", virtfsOptions}...)
			mounts = append(mounts, machine.Mount{Type: MountType9p, Tag: tag, Source: source, Target: target, ReadOnly: readonly})
		}
	}
	v.Mounts = mounts
	v.UID = os.Getuid()

	// Add location of bootable image
	v.CmdLine = append(v.CmdLine, "-drive", "if=virtio,file="+v.getImageFile())
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
	if err := v.writeConfig(); err != nil {
		return false, fmt.Errorf("writing JSON file: %w", err)
	}
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
	originalDiskSize, err := getDiskSize(v.getImageFile())
	if err != nil {
		return false, err
	}

	if err := v.resizeDisk(opts.DiskSize, originalDiskSize>>(10*3)); err != nil {
		return false, err
	}
	// If the user provides an ignition file, we need to
	// copy it into the conf dir
	if len(opts.IgnitionPath) > 0 {
		inputIgnition, err := ioutil.ReadFile(opts.IgnitionPath)
		if err != nil {
			return false, err
		}
		return false, ioutil.WriteFile(v.getIgnitionFile(), inputIgnition, 0644)
	}
	// Write the ignition file
	ign := machine.DynamicIgnition{
		Name:      opts.Username,
		Key:       key,
		VMName:    v.Name,
		TimeZone:  opts.TimeZone,
		WritePath: v.getIgnitionFile(),
		UID:       v.UID,
	}
	err = machine.NewIgnitionFile(ign)
	return err == nil, err
}

func (v *MachineVM) Set(_ string, opts machine.SetOptions) ([]error, error) {
	// If one setting fails to be applied, the others settings will not fail and still be applied.
	// The setting(s) that failed to be applied will have its errors returned in setErrors
	var setErrors []error

	state, err := v.State(false)
	if err != nil {
		return setErrors, err
	}

	if state == machine.Running {
		suffix := ""
		if v.Name != machine.DefaultMachineName {
			suffix = " " + v.Name
		}
		return setErrors, errors.Errorf("cannot change settings while the vm is running, run 'podman machine stop%s' first", suffix)
	}

	if opts.Rootful != nil && v.Rootful != *opts.Rootful {
		if err := v.setRootful(*opts.Rootful); err != nil {
			setErrors = append(setErrors, errors.Wrapf(err, "failed to set rootful option"))
		} else {
			v.Rootful = *opts.Rootful
		}
	}

	if opts.CPUs != nil && v.CPUs != *opts.CPUs {
		v.CPUs = *opts.CPUs
		v.editCmdLine("-smp", strconv.Itoa(int(v.CPUs)))
	}

	if opts.Memory != nil && v.Memory != *opts.Memory {
		v.Memory = *opts.Memory
		v.editCmdLine("-m", strconv.Itoa(int(v.Memory)))
	}

	if opts.DiskSize != nil && v.DiskSize != *opts.DiskSize {
		if err := v.resizeDisk(*opts.DiskSize, v.DiskSize); err != nil {
			setErrors = append(setErrors, errors.Wrapf(err, "failed to resize disk"))
		} else {
			v.DiskSize = *opts.DiskSize
		}
	}

	err = v.writeConfig()
	if err != nil {
		setErrors = append(setErrors, err)
	}

	if len(setErrors) > 0 {
		return setErrors, setErrors[0]
	}

	return setErrors, nil
}

// Start executes the qemu command line and forks it
func (v *MachineVM) Start(name string, _ machine.StartOptions) error {
	var (
		conn           net.Conn
		err            error
		qemuSocketConn net.Conn
		wait           = time.Millisecond * 500
	)

	if v.Starting {
		return fmt.Errorf("machine %q is already in the process of being started", v.Name)
	}

	v.Starting = true
	if err := v.writeConfig(); err != nil {
		return fmt.Errorf("writing JSON file: %w", err)
	}
	defer func() {
		v.Starting = false
		if err := v.writeConfig(); err != nil {
			logrus.Errorf("Writing JSON file: %v", err)
		}
	}()
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

	// If the qemusocketpath exists and the vm is off/down, we should rm
	// it before the dial as to avoid a segv
	if err := v.QMPMonitor.Address.Delete(); err != nil {
		return err
	}
	for i := 0; i < 6; i++ {
		qemuSocketConn, err = net.Dial("unix", v.QMPMonitor.Address.GetPath())
		if err == nil {
			break
		}
		time.Sleep(wait)
		wait++
	}
	if err != nil {
		return err
	}
	defer qemuSocketConn.Close()

	fd, err := qemuSocketConn.(*net.UnixConn).File()
	if err != nil {
		return err
	}
	defer fd.Close()
	dnr, err := os.OpenFile("/dev/null", os.O_RDONLY, 0755)
	if err != nil {
		return err
	}
	defer dnr.Close()
	dnw, err := os.OpenFile("/dev/null", os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer dnw.Close()

	attr := new(os.ProcAttr)
	files := []*os.File{dnr, dnw, dnw, fd}
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
			return errors.Wrapf(err, "unable to execute %q", cmd)
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
	defer conn.Close()
	_, err = bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return err
	}
	if len(v.Mounts) > 0 {
		state, err := v.State(true)
		if err != nil {
			return err
		}
		listening := v.isListening()
		for state != machine.Running || !listening {
			time.Sleep(100 * time.Millisecond)
			state, err = v.State(true)
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

func (v *MachineVM) checkStatus(monitor *qmp.SocketMonitor) (machine.Status, error) {
	// this is the format returned from the monitor
	// {"return": {"status": "running", "singlestep": false, "running": true}}

	type statusDetails struct {
		Status   string `json:"status"`
		Step     bool   `json:"singlestep"`
		Running  bool   `json:"running"`
		Starting bool   `json:"starting"`
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
func (v *MachineVM) Stop(_ string, _ machine.StopOptions) error {
	var disconnected bool
	// check if the qmp socket is there. if not, qemu instance is gone
	if _, err := os.Stat(v.QMPMonitor.Address.GetPath()); os.IsNotExist(err) {
		// Right now it is NOT an error to stop a stopped machine
		logrus.Debugf("QMP monitor socket %v does not exist", v.QMPMonitor.Address)
		return nil
	}
	qmpMonitor, err := qmp.NewSocketMonitor(v.QMPMonitor.Network, v.QMPMonitor.Address.GetPath(), v.QMPMonitor.Timeout)
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

	if _, err := os.Stat(v.PidFilePath.GetPath()); os.IsNotExist(err) {
		return nil
	}
	pidString, err := v.PidFilePath.Read()
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

	v.LastUp = time.Now()
	if err := v.writeConfig(); err != nil { // keep track of last up
		return err
	}
	// Kill the process
	if err := p.Kill(); err != nil {
		return err
	}
	// Remove the pidfile
	if err := v.PidFilePath.Delete(); err != nil {
		return err
	}
	// Remove socket
	if err := v.QMPMonitor.Address.Delete(); err != nil {
		return err
	}

	if err := qmpMonitor.Disconnect(); err != nil {
		// FIXME: this error should probably be returned
		return nil // nolint: nilerr
	}

	disconnected = true
	waitInternal := 250 * time.Millisecond
	for i := 0; i < 5; i++ {
		state, err := v.State(false)
		if err != nil {
			return err
		}
		if state != machine.Running {
			break
		}
		time.Sleep(waitInternal)
		waitInternal *= 2
	}

	return v.ReadySocket.Delete()
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
	if _, err := os.Stat(rtDir); os.IsNotExist(err) {
		if err := os.MkdirAll(rtDir, 0755); err != nil {
			return Monitor{}, err
		}
	}
	if timeout == 0 {
		timeout = defaultQMPTimeout
	}
	address, err := machine.NewMachineFile(filepath.Join(rtDir, "qmp_"+name+".sock"), nil)
	if err != nil {
		return Monitor{}, err
	}
	monitor := Monitor{
		Network: network,
		Address: *address,
		Timeout: timeout,
	}
	return monitor, nil
}

// Remove deletes all the files associated with a machine including ssh keys, the image itself
func (v *MachineVM) Remove(_ string, opts machine.RemoveOptions) (string, func() error, error) {
	var (
		files []string
	)

	// cannot remove a running vm unless --force is used
	state, err := v.State(false)
	if err != nil {
		return "", nil, err
	}
	if state == machine.Running {
		if !opts.Force {
			return "", nil, errors.Errorf("running vm %q cannot be destroyed", v.Name)
		}
		err := v.Stop(v.Name, machine.StopOptions{})
		if err != nil {
			return "", nil, err
		}
	}

	// Collect all the files that need to be destroyed
	if !opts.SaveKeys {
		files = append(files, v.IdentityPath, v.IdentityPath+".pub")
	}
	if !opts.SaveIgnition {
		files = append(files, v.getIgnitionFile())
	}
	if !opts.SaveImage {
		files = append(files, v.getImageFile())
	}
	socketPath, err := v.forwardSocketPath()
	if err != nil {
		return "", nil, err
	}
	if socketPath.Symlink != nil {
		files = append(files, *socketPath.Symlink)
	}
	files = append(files, socketPath.Path)
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

	// remove socket and pid file if any: warn at low priority if things fail
	// Remove the pidfile
	if err := v.PidFilePath.Delete(); err != nil {
		logrus.Debugf("Error while removing pidfile: %v", err)
	}
	// Remove socket
	if err := v.QMPMonitor.Address.Delete(); err != nil {
		logrus.Debugf("Error while removing podman-machine-socket: %v", err)
	}

	confirmationMessage += "\n"
	return confirmationMessage, func() error {
		for _, f := range files {
			if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
				logrus.Error(err)
			}
		}
		return nil
	}, nil
}

func (v *MachineVM) State(bypass bool) (machine.Status, error) {
	// Check if qmp socket path exists
	if _, err := os.Stat(v.QMPMonitor.Address.GetPath()); os.IsNotExist(err) {
		return "", nil
	}
	err := v.update()
	if err != nil {
		return "", err
	}
	// Check if we can dial it
	if v.Starting && !bypass {
		return machine.Starting, nil
	}
	monitor, err := qmp.NewSocketMonitor(v.QMPMonitor.Network, v.QMPMonitor.Address.GetPath(), v.QMPMonitor.Timeout)
	if err != nil {
		// FIXME: this error should probably be returned
		return "", err
	}
	if err := monitor.Connect(); err != nil {
		return "", err
	}
	defer func() {
		if err := monitor.Disconnect(); err != nil {
			logrus.Error(err)
		}
	}()
	// If there is a monitor, lets see if we can query state
	return v.checkStatus(monitor)
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
func (v *MachineVM) SSH(_ string, opts machine.SSHOptions) error {
	state, err := v.State(true)
	if err != nil {
		return err
	}
	if state != machine.Running {
		return errors.Errorf("vm %q is not running", v.Name)
	}

	username := opts.Username
	if username == "" {
		username = v.RemoteUsername
	}

	sshDestination := username + "@localhost"
	port := strconv.Itoa(v.Port)

	args := []string{"-i", v.IdentityPath, "-p", port, sshDestination, "-o", "UserKnownHostsFile=/dev/null", "-o", "StrictHostKeyChecking=no"}
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
			b, err := ioutil.ReadFile(fullPath)
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

			if !vm.LastUp.IsZero() { // this means we have already written a time to the config
				listEntry.LastUp = vm.LastUp
			} else { // else we just created the machine AKA last up = created time
				listEntry.LastUp = vm.Created
				vm.LastUp = listEntry.LastUp
				if err := vm.writeConfig(); err != nil {
					return err
				}
			}
			switch state {
			case machine.Running:
				listEntry.Running = true
			case machine.Starting:
				listEntry.Starting = true
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
func (p *Provider) CheckExclusiveActiveVM() (bool, string, error) {
	vms, err := getVMInfos()
	if err != nil {
		return false, "", errors.Wrap(err, "error checking VM active")
	}
	for _, vm := range vms {
		if vm.Running || vm.Starting {
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

	attr := new(os.ProcAttr)
	dnr, err := os.OpenFile("/dev/null", os.O_RDONLY, 0755)
	if err != nil {
		return "", noForwarding, err
	}
	dnw, err := os.OpenFile("/dev/null", os.O_WRONLY, 0755)
	if err != nil {
		return "", noForwarding, err
	}

	defer dnr.Close()
	defer dnw.Close()

	attr.Files = []*os.File{dnr, dnw, dnw}
	cmd := []string{binary}
	cmd = append(cmd, []string{"-listen-qemu", fmt.Sprintf("unix://%s", v.QMPMonitor.Address.GetPath()), "-pid-file", v.PidFilePath.GetPath()}...)
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
	return forwardSock, state, errors.Wrapf(err, "unable to execute: %q", cmd)
}

func (v *MachineVM) setupAPIForwarding(cmd []string) ([]string, string, apiForwardingState) {
	socket, err := v.forwardSocketPath()

	if err != nil {
		return cmd, "", noForwarding
	}

	destSock := fmt.Sprintf("/run/user/%d/podman/podman.sock", v.UID)
	forwardUser := "core"

	if v.Rootful {
		destSock = "/run/podman/podman.sock"
		forwardUser = "root"
	}

	cmd = append(cmd, []string{"-forward-sock", socket.GetPath()}...)
	cmd = append(cmd, []string{"-forward-dest", destSock}...)
	cmd = append(cmd, []string{"-forward-user", forwardUser}...)
	cmd = append(cmd, []string{"-forward-identity", v.IdentityPath}...)

	// The linking pattern is /var/run/docker.sock -> user global sock (link) -> machine sock (socket)
	// This allows the helper to only have to maintain one constant target to the user, which can be
	// repositioned without updating docker.sock.

	link, err := v.userGlobalSocketLink()
	if err != nil {
		return cmd, socket.GetPath(), machineLocal
	}

	if !dockerClaimSupported() {
		return cmd, socket.GetPath(), claimUnsupported
	}

	if !dockerClaimHelperInstalled() {
		return cmd, socket.GetPath(), notInstalled
	}

	if !alreadyLinked(socket.GetPath(), link) {
		if checkSockInUse(link) {
			return cmd, socket.GetPath(), machineLocal
		}

		_ = os.Remove(link)
		if err = os.Symlink(socket.GetPath(), link); err != nil {
			logrus.Warnf("could not create user global API forwarding link: %s", err.Error())
			return cmd, socket.GetPath(), machineLocal
		}
	}

	if !alreadyLinked(link, dockerSock) {
		if checkSockInUse(dockerSock) {
			return cmd, socket.GetPath(), machineLocal
		}

		if !claimDockerSock() {
			logrus.Warn("podman helper is installed, but was not able to claim the global docker sock")
			return cmd, socket.GetPath(), machineLocal
		}
	}

	return cmd, dockerSock, dockerGlobal
}

func (v *MachineVM) isIncompatible() bool {
	return v.UID == -1
}

func (v *MachineVM) userGlobalSocketLink() (string, error) {
	path, err := machine.GetDataDir(v.Name)
	if err != nil {
		logrus.Errorf("Resolving data dir: %s", err.Error())
		return "", err
	}
	// User global socket is located in parent directory of machine dirs (one per user)
	return filepath.Join(filepath.Dir(path), "podman.sock"), err
}

func (v *MachineVM) forwardSocketPath() (*machine.VMFile, error) {
	sockName := "podman.sock"
	path, err := machine.GetDataDir(v.Name)
	if err != nil {
		logrus.Errorf("Resolving data dir: %s", err.Error())
		return nil, err
	}
	return machine.NewMachineFile(filepath.Join(path, sockName), &sockName)
}

func (v *MachineVM) setConfigPath() error {
	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return err
	}

	configPath, err := machine.NewMachineFile(filepath.Join(vmConfigDir, v.Name)+".json", nil)
	if err != nil {
		return err
	}
	v.ConfigPath = *configPath
	return nil
}

func (v *MachineVM) setReadySocket() error {
	readySocketName := v.Name + "_ready.sock"
	rtPath, err := getRuntimeDir()
	if err != nil {
		return err
	}
	virtualSocketPath, err := machine.NewMachineFile(filepath.Join(rtPath, "podman", readySocketName), &readySocketName)
	if err != nil {
		return err
	}
	v.ReadySocket = *virtualSocketPath
	return nil
}

func (v *MachineVM) setPIDSocket() error {
	rtPath, err := getRuntimeDir()
	if err != nil {
		return err
	}
	if !rootless.IsRootless() {
		rtPath = "/run"
	}
	pidFileName := fmt.Sprintf("%s.pid", v.Name)
	socketDir := filepath.Join(rtPath, "podman")
	pidFilePath, err := machine.NewMachineFile(filepath.Join(socketDir, pidFileName), &pidFileName)
	if err != nil {
		return err
	}
	v.PidFilePath = *pidFilePath
	return nil
}

// Deprecated: getSocketandPid is being replace by setPIDSocket and
// machinefiles.
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
				if err != nil {
					return nil, err
				}
				if err := con.SetDeadline(time.Now().Add(apiUpTimeout)); err != nil {
					return nil, err
				}
				return con, nil
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

// update returns the content of the VM's
// configuration file in json
func (v *MachineVM) update() error {
	if err := v.setConfigPath(); err != nil {
		return err
	}
	b, err := v.ConfigPath.Read()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.Wrap(machine.ErrNoSuchVM, v.Name)
		}
		return err
	}
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, v)
	if err != nil {
		err = migrateVM(v.ConfigPath.GetPath(), b, v)
		if err != nil {
			return err
		}
	}
	return err
}

func (v *MachineVM) writeConfig() error {
	// Set the path of the configfile before writing to make
	// life easier down the line
	if err := v.setConfigPath(); err != nil {
		return err
	}
	// Write the JSON file
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(v.ConfigPath.GetPath(), b, 0644); err != nil {
		return err
	}
	return nil
}

// getImageFile wrapper returns the path to the image used
// to boot the VM
func (v *MachineVM) getImageFile() string {
	return v.ImagePath.GetPath()
}

// getIgnitionFile wrapper returns the path to the ignition file
func (v *MachineVM) getIgnitionFile() string {
	return v.IgnitionFile.GetPath()
}

// Inspect returns verbose detail about the machine
func (v *MachineVM) Inspect() (*machine.InspectInfo, error) {
	state, err := v.State(false)
	if err != nil {
		return nil, err
	}

	return &machine.InspectInfo{
		ConfigPath: v.ConfigPath,
		Created:    v.Created,
		Image:      v.ImageConfig,
		LastUp:     v.LastUp,
		Name:       v.Name,
		Resources:  v.ResourceConfig,
		SSHConfig:  v.SSHConfig,
		State:      state,
	}, nil
}

// resizeDisk increases the size of the machine's disk in GB.
func (v *MachineVM) resizeDisk(diskSize uint64, oldSize uint64) error {
	// Resize the disk image to input disk size
	// only if the virtualdisk size is less than
	// the given disk size
	if diskSize < oldSize {
		return errors.Errorf("new disk size must be larger than current disk size: %vGB", oldSize)
	}

	// Find the qemu executable
	cfg, err := config.Default()
	if err != nil {
		return err
	}
	resizePath, err := cfg.FindHelperBinary("qemu-img", true)
	if err != nil {
		return err
	}
	resize := exec.Command(resizePath, []string{"resize", v.getImageFile(), strconv.Itoa(int(diskSize)) + "G"}...)
	resize.Stdout = os.Stdout
	resize.Stderr = os.Stderr
	if err := resize.Run(); err != nil {
		return errors.Errorf("resizing image: %q", err)
	}

	return nil
}

func (v *MachineVM) setRootful(rootful bool) error {
	changeCon, err := machine.AnyConnectionDefault(v.Name, v.Name+"-root")
	if err != nil {
		return err
	}

	if changeCon {
		newDefault := v.Name
		if rootful {
			newDefault += "-root"
		}
		err := machine.ChangeDefault(newDefault)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *MachineVM) editCmdLine(flag string, value string) {
	found := false
	for i, val := range v.CmdLine {
		if val == flag {
			found = true
			v.CmdLine[i+1] = value
			break
		}
	}
	if !found {
		v.CmdLine = append(v.CmdLine, []string{flag, value}...)
	}
}

// RemoveAndCleanMachines removes all machine and cleans up any other files associatied with podman machine
func (p *Provider) RemoveAndCleanMachines() error {
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
		err := os.RemoveAll(dataDir)
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
		err := os.RemoveAll(confDir)
		if err != nil {
			if prevErr != nil {
				logrus.Error(prevErr)
			}
			prevErr = err
		}
	}
	return prevErr
}
