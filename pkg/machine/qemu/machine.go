//go:build (amd64 && !windows) || (arm64 && !windows)
// +build amd64,!windows arm64,!windows

package qemu

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/machine"
	"github.com/containers/podman/v3/pkg/rootless"
	"github.com/containers/podman/v3/utils"
	"github.com/containers/storage/pkg/homedir"
	"github.com/digitalocean/go-qemu/qmp"
	"github.com/moby/term"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	// vmtype refers to qemu (vs libvirt, krun, etc)
	vmtype = "qemu"
)

// NewMachine initializes an instance of a virtual machine based on the qemu
// virtualization.
func NewMachine(opts machine.InitOptions) (machine.VM, error) {
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

	// An image was specified
	if len(opts.ImagePath) > 0 {
		vm.ImagePath = opts.ImagePath
	}

	// Assign remote user name. if not provided, use default
	vm.RemoteUsername = opts.Username
	if len(vm.RemoteUsername) < 1 {
		vm.RemoteUsername = defaultRemoteUser
	}

	// Add a random port for ssh
	port, err := utils.GetRandomPort()
	if err != nil {
		return nil, err
	}
	vm.Port = port

	vm.CPUs = opts.CPUS
	vm.Memory = opts.Memory

	// Look up the executable
	execPath, err := exec.LookPath(QemuCommand)
	if err != nil {
		return nil, err
	}
	cmd := append([]string{execPath})
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
func LoadVMByName(name string) (machine.VM, error) {
	vm := new(MachineVM)
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
	logrus.Debug(vm.CmdLine)
	return vm, err
}

// Init writes the json configuration file to the filesystem for
// other verbs (start, stop)
func (v *MachineVM) Init(opts machine.InitOptions) error {
	var (
		key string
	)

	sshDir := filepath.Join(homedir.Get(), ".ssh")
	// GetConfDir creates the directory so no need to check for
	// its existence
	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return err
	}
	jsonFile := filepath.Join(vmConfigDir, v.Name) + ".json"
	v.IdentityPath = filepath.Join(sshDir, v.Name)

	switch opts.ImagePath {
	case "testing", "stable", "":
		// Get image as usual
		dd, err := machine.NewFcosDownloader(vmtype, v.Name, opts.ImagePath)
		if err != nil {
			return err
		}
		v.ImagePath = dd.Get().LocalUncompressedFile
		if err := dd.DownloadImage(); err != nil {
			return err
		}
	default:
		// The user has provided an alternate image which can be a file path
		// or URL.
		g, err := machine.NewGenericDownloader(vmtype, v.Name, opts.ImagePath)
		if err != nil {
			return err
		}
		v.ImagePath = g.Get().LocalUncompressedFile
		if err := g.DownloadImage(); err != nil {
			return err
		}
	}
	// Add arch specific options including image location
	v.CmdLine = append(v.CmdLine, v.addArchOptions()...)

	// Add location of bootable image
	v.CmdLine = append(v.CmdLine, "-drive", "if=virtio,file="+v.ImagePath)
	// This kind of stinks but no other way around this r/n
	if len(opts.IgnitionPath) < 1 {
		uri := machine.SSHRemoteConnection.MakeSSHURL("localhost", "/run/user/1000/podman/podman.sock", strconv.Itoa(v.Port), v.RemoteUsername)
		if err := machine.AddConnection(&uri, v.Name, filepath.Join(sshDir, v.Name), opts.IsDefault); err != nil {
			return err
		}

		uriRoot := machine.SSHRemoteConnection.MakeSSHURL("localhost", "/run/podman/podman.sock", strconv.Itoa(v.Port), "root")
		if err := machine.AddConnection(&uriRoot, v.Name+"-root", filepath.Join(sshDir, v.Name), opts.IsDefault); err != nil {
			return err
		}
	} else {
		fmt.Println("An ignition path was provided.  No SSH connection was added to Podman")
	}
	// Write the JSON file
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(jsonFile, b, 0644); err != nil {
		return err
	}

	// User has provided ignition file so keygen
	// will be skipped.
	if len(opts.IgnitionPath) < 1 {
		key, err = machine.CreateSSHKeys(v.IdentityPath)
		if err != nil {
			return err
		}
	}
	// Run arch specific things that need to be done
	if err := v.prepare(); err != nil {
		return err
	}

	originalDiskSize, err := getDiskSize(v.ImagePath)
	if err != nil {
		return err
	}
	// Resize the disk image to input disk size
	// only if the virtualdisk size is less than
	// the given disk size
	if opts.DiskSize<<(10*3) > originalDiskSize {
		resize := exec.Command("qemu-img", []string{"resize", v.ImagePath, strconv.Itoa(int(opts.DiskSize)) + "G"}...)
		resize.Stdout = os.Stdout
		resize.Stderr = os.Stderr
		if err := resize.Run(); err != nil {
			return errors.Errorf("error resizing image: %q", err)
		}
	}

	if !opts.NoLink {
		err = createDockerSock(v)
		if err != nil {
			return err
		}
	}

	// If the user provides an ignition file, we need to
	// copy it into the conf dir
	if len(opts.IgnitionPath) > 0 {
		inputIgnition, err := ioutil.ReadFile(opts.IgnitionPath)
		if err != nil {
			return err
		}
		return ioutil.WriteFile(v.IgnitionFilePath, inputIgnition, 0644)
	}

	// Write the ignition file
	ign := machine.DynamicIgnition{
		Name:      opts.Username,
		Key:       key,
		VMName:    v.Name,
		WritePath: v.IgnitionFilePath,
	}
	return machine.NewIgnitionFile(ign)
}

// Start executes the qemu command line and forks it
func (v *MachineVM) Start(name string, startOptions machine.StartOptions) error {
	var (
		conn           net.Conn
		err            error
		qemuSocketConn net.Conn
	)

	if err := v.startHostNetworking(); err != nil {
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

	qemuSocketConn, err = socketWait(qemuSocketPath, false)
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
	if logrus.GetLevel() != logrus.DebugLevel {
		cmd = append(cmd, "-display", "none")
	}

	_, err = os.StartProcess(v.CmdLine[0], cmd, attr)
	if err != nil {
		return err
	}
	fmt.Println("Waiting for VM ...")
	socketPath, err := getRuntimeDir()
	if err != nil {
		return err
	}

	// The socket is not made until the qemu process is running so here
	// we do a backoff waiting for it.  Once we have a conn, we break and
	// then wait to read it.
	conn, err = socketWait(filepath.Join(socketPath, "podman", v.Name+"_ready.sock"), false)
	if err != nil {
		return err
	}
	_, err = bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return err
	}

	var (
		rootfullSocket string
		rootlessSocket string
	)
	if !startOptions.NoCompat {
		err = createDockerSock(v)
		if err != nil {
			return err
		}
		if !startOptions.NoLink {
			rootfullSocket, rootlessSocket, err = createCompatibilityProxies(v)
			if err != nil {
				return err
			}
		}
	}

	fmt.Printf("Machine %q started successfully!\n\n", v.Name)
	fmt.Printf("Podman Clients\n")
	fmt.Printf("--------------\n")
	fmt.Printf("Podman clients can now access this machine using standard podman commands.\n")
	fmt.Printf("For example, to run a date command on a *rootless* container:\n")
	fmt.Printf("\n    podman run ubi8/ubi-micro date\n")
	fmt.Printf("\nTo bind port 80 using a *root* container:\n")
	fmt.Printf("\n    podman -c podman-machine-default-root run -dt -p 80:80/tcp docker.io/library/httpd\n\n")

	if !startOptions.NoCompat {
		fmt.Printf("Docker API Clients\n")
		fmt.Printf("------------------\n")
		if active, _, _ := verifyDockerSock(rootfullSocket); active {
			fmt.Printf("Compatibility socket link is present. Docker API clients require no special environment for *root* containers.\n\n")
		} else {
			fmt.Printf("Compatibility socket link is not present, rerun 'podman machine init' to create.\n")
			fmt.Printf("*Root* containers can still be accessed using:\n\n")
			fmt.Printf("    export DOCKER_HOST=unix://%s\n\n", rootfullSocket)
		}
		fmt.Printf("Docker API clients can also access *rootless* podman with the following environment:\n\n")
		fmt.Printf("    export DOCKER_HOST=unix://%s\n\n", rootlessSocket)
	}

	return nil
}

func verifyDockerSock(rootfullSocket string) (bool, bool, string) {
	target, err := os.Readlink("/var/run/docker.sock")
	return target == rootfullSocket, errors.Is(err, fs.ErrNotExist), target
}

func createDockerSock(v *MachineVM) error {
	var err error

	rootfullSocket, _, err := v.getUnixSocketAndPID(true)
	if err != nil {
		err = errors.Errorf("Unable to determine runtime env for socket: %q", err)
	}

	skip, safe, current := verifyDockerSock(rootfullSocket)
	if skip {
		return nil
	}

	if !term.IsTerminal(os.Stdin.Fd()) {
		fmt.Println("No terminal present, can't prompt to create link")
		return nil
	}

	if !safe {
		fmt.Printf("WARNING: Docker socket (/var/run/docker.sock) already exists (points to %q\n)", current)
		fmt.Print("Overwrite to point to podman? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		answer, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.ToLower(answer)[0] != 'y' {
			return nil
		}
	}

	fmt.Println("Creating /var/run/docker.sock compatibility link (you might be prompted for your password)")
	cmd := exec.Command("/usr/bin/sudo", "/bin/ln", "-fs", rootfullSocket, "/var/run/docker.sock")
	if cmd.Run() != nil {
		err = errors.Errorf("Sudo failed creating link. %q", err)
	}

	return err
}

func socketWait(socket string, close bool) (net.Conn, error) {
	return socketWaitS("unix", socket, close, false)
}

func socketWaitS(schema string, socket string, close bool, read bool) (net.Conn, error) {
	wait := time.Millisecond * 50

	var (
		err  error
		conn net.Conn
	)
	for i := 0; i < 8; i++ {
		conn, err = net.Dial(schema, socket)
		if err == nil {
			if read {
				buffer := make([]byte, 10)
				_, err = conn.Read(buffer)
			}
			if close {
				conn.Close()
			}
			if err == nil {
				break
			}
		}
		time.Sleep(wait)
		wait *= 2
	}

	return conn, err
}

func createCompatibilityProxies(v *MachineVM) (string, string, error) {
	fmt.Println("Waiting on SSH to come up..")

	// FIXME gvproxy generates some noise, try to wait in advance to mute the proxy failures
	time.Sleep(1000 * time.Millisecond)

	_, err := socketWaitS("tcp", fmt.Sprintf("localhost:%d", v.Port), true, true)
	if err != nil {
		return "", "", err
	}

	fmt.Println("Waiting on initial proxy connections..")

	for i := 0; i < 2; i++ {
		if err := v.startUnixProxy(i == 1); err != nil {
			return "", "", errors.Errorf("Unable to start unix socket proxy: %q", err)
		}
	}

	rootlessSocket, _, _ := v.getUnixSocketAndPID(false)
	rootfullSocket, _, _ := v.getUnixSocketAndPID(true)

	for _, s := range []string{rootfullSocket, rootlessSocket} {
		err := v.verifyProxyConnection(s)
		if err != nil {
			return "", "", err
		}
	}

	return rootfullSocket, rootlessSocket, nil
}

// Stop uses the qmp monitor to call a system_powerdown
func (v *MachineVM) Stop(name string, _ machine.StopOptions) error {
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
		if err := qmpMonitor.Disconnect(); err != nil {
			logrus.Error(err)
		}
	}()
	if _, err = qmpMonitor.Run(input); err != nil {
		return err
	}
	qemuSocketFile, pidFile, err := v.getSocketandPid()
	cleanupProcess(pidFile, qemuSocketFile)
	if err != nil {
		return err
	}

	for i := 0; i < 2; i++ {
		unixSocketFile, pidFile, err := v.getUnixSocketAndPID(i == 1)
		cleanupProcess(pidFile, unixSocketFile)
		if err != nil {
			return err
		}
	}

	return nil
}

func cleanupProcess(pidFile string, socketFile string) error {
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		logrus.Infof("pid file %s does not exist", pidFile)
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
	os.Remove(socketFile)
	if err != nil {
		return nil
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
	if v.isRunning() {
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

func (v *MachineVM) isRunning() bool {
	// Check if qmp socket path exists
	if _, err := os.Stat(v.QMPMonitor.Address); os.IsNotExist(err) {
		return false
	}
	// Check if we can dial it
	if _, err := qmp.NewSocketMonitor(v.QMPMonitor.Network, v.QMPMonitor.Address, v.QMPMonitor.Timeout); err != nil {
		return false
	}
	return true
}

// SSH opens an interactive SSH session to the vm specified.
// Added ssh function to VM interface: pkg/machine/config/go : line 58
func (v *MachineVM) SSH(name string, opts machine.SSHOptions) error {
	if !v.isRunning() {
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
	diskInfo := exec.Command("qemu-img", "info", "--output", "json", path)
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
func List(_ machine.ListOptions) ([]*machine.ListResponse, error) {
	return GetVMInfos()
}

func GetVMInfos() ([]*machine.ListResponse, error) {
	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return nil, err
	}

	var listed []*machine.ListResponse

	if err = filepath.Walk(vmConfigDir, func(path string, info os.FileInfo, err error) error {
		vm := new(MachineVM)
		if strings.HasSuffix(info.Name(), ".json") {
			fullPath := filepath.Join(vmConfigDir, info.Name())
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
			listEntry.VMType = "qemu"
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
			if vm.isRunning() {
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

func IsValidVMName(name string) (bool, error) {
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

// CheckActiveVM checks if there is a VM already running
func CheckActiveVM() (bool, string, error) {
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
func (v *MachineVM) startHostNetworking() error {
	cfg, err := config.Default()
	if err != nil {
		return err
	}
	binary, err := cfg.FindHelperBinary(machine.ForwarderBinaryName, false)
	if err != nil {
		return err
	}

	// Listen on all at port 7777 for setting up and tearing
	// down forwarding
	listenSocket := "tcp://0.0.0.0:7777"
	qemuSocket, pidFile, err := v.getSocketandPid()
	if err != nil {
		return err
	}
	attr := new(os.ProcAttr)
	// Pass on stdin, stdout, stderr
	files := []*os.File{os.Stdin, os.Stdout, os.Stderr}
	attr.Files = files
	cmd := []string{binary}
	cmd = append(cmd, []string{"-listen", listenSocket, "-listen-qemu", fmt.Sprintf("unix://%s", qemuSocket), "-pid-file", pidFile}...)
	// Add the ssh port
	cmd = append(cmd, []string{"-ssh-port", fmt.Sprintf("%d", v.Port)}...)
	if logrus.GetLevel() == logrus.DebugLevel {
		cmd = append(cmd, "--debug")
		fmt.Println(cmd)
	}
	_, err = os.StartProcess(cmd[0], cmd, attr)
	return err
}

func (v *MachineVM) startUnixProxy(root bool) error {
	binary, err := os.Executable()
	if err != nil {
		return err
	}

	unixSocket, pidFile, err := v.getUnixSocketAndPID(root)
	if err != nil {
		return err
	}
	attr := new(os.ProcAttr)
	// Pass on stdin, stdout, stderr
	files := []*os.File{os.Stdin, os.Stdout, os.Stderr}
	attr.Files = files
	cmd := []string{binary}
	if logrus.GetLevel() == logrus.DebugLevel {
		cmd = append(cmd, "--debug")
		fmt.Println(cmd)
	}
	suffix := ""
	if root {
		suffix = "-root"
	}
	name := fmt.Sprintf("%s%s", v.Name, suffix)
	cmd = append(cmd, []string{"-c", name, "system", "unix-proxy", "-q", "--pid-file", pidFile, "unix://" + unixSocket}...)
	_, err = os.StartProcess(cmd[0], cmd, attr)

	return err
}

func (v *MachineVM) verifyProxyConnection(unixSocket string) error {
	_, err := socketWait(unixSocket, true)
	if err != nil {
		return err
	}

	_, err = bindings.NewConnection(context.Background(), "unix://"+unixSocket)
	return err
}

func (v *MachineVM) getUnixSocketAndPID(root bool) (string, string, error) {
	rtPath, err := getRuntimeDir()
	if err != nil {
		return "", "", err
	}
	if !rootless.IsRootless() {
		rtPath = "/run"
	}
	suffix := ""
	if root {
		suffix = "-root"
	}
	socketDir := filepath.Join(rtPath, "podman")
	pidFile := filepath.Join(socketDir, fmt.Sprintf("%s%s-unix.pid", v.Name, suffix))
	unixSocket := filepath.Join(socketDir, fmt.Sprintf("%s%s-api.sock", v.Name, suffix))
	return unixSocket, pidFile, nil
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
