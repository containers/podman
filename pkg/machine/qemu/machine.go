//go:build amd64 || arm64
// +build amd64 arm64

package qemu

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containers/common/pkg/config"
	gvproxy "github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/define"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/storage/pkg/lockfile"
	"github.com/digitalocean/go-qemu/qmp"
	"github.com/sirupsen/logrus"
)

var (
	// vmtype refers to qemu (vs libvirt, krun, etc).
	// Could this be moved into  Provider
	vmtype = machine.QemuVirt
)

const (
	VolumeTypeVirtfs     = "virtfs"
	MountType9p          = "9p"
	dockerSock           = "/var/run/docker.sock"
	dockerConnectTimeout = 5 * time.Second
)

// qemuReadyUnit is a unit file that sets up the virtual serial device
// where when the VM is done configuring, it will send an ack
// so a listening host tknows it can begin interacting with it
const qemuReadyUnit = `[Unit]
Requires=dev-virtio\\x2dports-%s.device
After=remove-moby.service sshd.socket sshd.service
After=systemd-user-sessions.service
OnFailure=emergency.target
OnFailureJobMode=isolate
[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/bin/sh -c '/usr/bin/echo Ready >/dev/%s'
[Install]
RequiredBy=default.target
`

type MachineVM struct {
	// ConfigPath is the path to the configuration file
	ConfigPath define.VMFile
	// The command line representation of the qemu command
	CmdLine QemuCmd
	// HostUser contains info about host user
	machine.HostUser
	// ImageConfig describes the bootable image
	machine.ImageConfig
	// Mounts is the list of remote filesystems to mount
	Mounts []machine.Mount
	// Name of VM
	Name string
	// PidFilePath is the where the Proxy PID file lives
	PidFilePath define.VMFile
	// VMPidFilePath is the where the VM PID file lives
	VMPidFilePath define.VMFile
	// QMPMonitor is the qemu monitor object for sending commands
	QMPMonitor Monitor
	// ReadySocket tells host when vm is booted
	ReadySocket define.VMFile
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

	// User at runtime for serializing write operations.
	lock *lockfile.LockFile
}

type Monitor struct {
	//	Address portion of the qmp monitor (/tmp/tmp.sock)
	Address define.VMFile
	// Network portion of the qmp monitor (unix)
	Network string
	// Timeout in seconds for qmp monitor transactions
	Timeout time.Duration
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

	pidFilePath := define.VMFile{Path: pidFile}
	qmpMonitor := Monitor{
		Address: define.VMFile{Path: old.QMPMonitor.Address},
		Network: old.QMPMonitor.Network,
		Timeout: old.QMPMonitor.Timeout,
	}
	socketPath, err := getRuntimeDir()
	if err != nil {
		return err
	}
	virtualSocketPath := filepath.Join(socketPath, "podman", vm.Name+"_ready.sock")
	readySocket := define.VMFile{Path: virtualSocketPath}

	vm.HostUser = machine.HostUser{}
	vm.ImageConfig = machine.ImageConfig{}
	vm.ResourceConfig = machine.ResourceConfig{}
	vm.SSHConfig = machine.SSHConfig{}

	ignitionFilePath, err := define.NewMachineFile(old.IgnitionFilePath, nil)
	if err != nil {
		return err
	}
	imagePath, err := define.NewMachineFile(old.ImagePath, nil)
	if err != nil {
		return err
	}

	// setReadySocket will stick the entry into the new struct
	symlink := vm.Name + "_ready.sock"
	if err := machine.SetSocket(&vm.ReadySocket, machine.ReadySocketPath(socketPath+"/podman/", vm.Name), &symlink); err != nil {
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

	// Back up the original config file
	if err := os.Rename(configPath, configPath+".orig"); err != nil {
		return err
	}
	// Write the config file
	if err := vm.writeConfig(); err != nil {
		// If the config file fails to be written, put the original
		// config file back before erroring
		if renameError := os.Rename(configPath+".orig", configPath); renameError != nil {
			logrus.Warn(renameError)
		}
		return err
	}
	// Remove the backup file
	return os.Remove(configPath + ".orig")
}

// addMountsToVM converts the volumes passed through the CLI into the specified
// volume driver and adds them to the machine
func (v *MachineVM) addMountsToVM(opts machine.InitOptions) error {
	var volumeType string
	switch opts.VolumeDriver {
	// "" is the default volume driver
	case "virtfs", "":
		volumeType = VolumeTypeVirtfs
	default:
		return fmt.Errorf("unknown volume driver: %s", opts.VolumeDriver)
	}

	mounts := []machine.Mount{}
	for i, volume := range opts.Volumes {
		tag := fmt.Sprintf("vol%d", i)
		paths := pathsFromVolume(volume)
		source := extractSourcePath(paths)
		target := extractTargetPath(paths)
		readonly, securityModel := extractMountOptions(paths)
		if volumeType == VolumeTypeVirtfs {
			v.CmdLine.SetVirtfsMount(source, tag, securityModel, readonly)
			mounts = append(mounts, machine.Mount{Type: MountType9p, Tag: tag, Source: source, Target: target, ReadOnly: readonly})
		}
	}
	v.Mounts = mounts
	return nil
}

// Init writes the json configuration file to the filesystem for
// other verbs (start, stop)
func (v *MachineVM) Init(opts machine.InitOptions) (bool, error) {
	var (
		key string
		err error
	)

	// cleanup half-baked files if init fails at any point
	callbackFuncs := machine.InitCleanup()
	defer callbackFuncs.CleanIfErr(&err)
	go callbackFuncs.CleanOnSignal()

	v.IdentityPath = util.GetIdentityPath(v.Name)
	v.Rootful = opts.Rootful

	imagePath, strm, err := machine.Pull(opts.ImagePath, opts.Name, VirtualizationProvider())
	if err != nil {
		return false, err
	}

	//  By this time, image should be had and uncompressed
	callbackFuncs.Add(imagePath.Delete)

	// Assign values about the download
	v.ImagePath = *imagePath
	v.ImageStream = strm.String()

	if err = v.addMountsToVM(opts); err != nil {
		return false, err
	}

	v.UID = os.Getuid()

	// Add location of bootable image
	v.CmdLine.SetBootableImage(v.getImageFile())

	if err = machine.AddSSHConnectionsToPodmanSocket(
		v.UID,
		v.Port,
		v.IdentityPath,
		v.Name,
		v.RemoteUsername,
		opts,
	); err != nil {
		return false, err
	}
	callbackFuncs.Add(v.removeSystemConnections)

	// Write the JSON file
	if err = v.writeConfig(); err != nil {
		return false, fmt.Errorf("writing JSON file: %w", err)
	}
	callbackFuncs.Add(v.ConfigPath.Delete)

	// User has provided ignition file so keygen
	// will be skipped.
	if len(opts.IgnitionPath) < 1 {
		key, err = machine.CreateSSHKeys(v.IdentityPath)
		if err != nil {
			return false, err
		}
		callbackFuncs.Add(v.removeSSHKeys)
	}
	// Run arch specific things that need to be done
	if err = v.prepare(); err != nil {
		return false, err
	}
	originalDiskSize, err := getDiskSize(v.getImageFile())
	if err != nil {
		return false, err
	}

	if err = v.resizeDisk(opts.DiskSize, originalDiskSize>>(10*3)); err != nil {
		return false, err
	}

	if opts.UserModeNetworking != nil && !*opts.UserModeNetworking {
		logrus.Warn("ignoring init option to disable user-mode networking: this mode is not supported by the QEMU backend")
	}

	builder := machine.NewIgnitionBuilder(machine.DynamicIgnition{
		Name:      opts.Username,
		Key:       key,
		VMName:    v.Name,
		VMType:    machine.QemuVirt,
		TimeZone:  opts.TimeZone,
		WritePath: v.getIgnitionFile(),
		UID:       v.UID,
		Rootful:   v.Rootful,
	})

	// If the user provides an ignition file, we need to
	// copy it into the conf dir
	if len(opts.IgnitionPath) > 0 {
		return false, builder.BuildWithIgnitionFile(opts.IgnitionPath)
	}

	if err := builder.GenerateIgnitionConfig(); err != nil {
		return false, err
	}

	readyUnit := machine.Unit{
		Enabled:  machine.BoolToPtr(true),
		Name:     "ready.service",
		Contents: machine.StrToPtr(fmt.Sprintf(qemuReadyUnit, "vport1p1", "vport1p1")),
	}
	builder.WithUnit(readyUnit)

	err = builder.Build()
	callbackFuncs.Add(v.IgnitionFile.Delete)

	return err == nil, err
}

func (v *MachineVM) removeSSHKeys() error {
	if err := os.Remove(fmt.Sprintf("%s.pub", v.IdentityPath)); err != nil {
		logrus.Error(err)
	}
	return os.Remove(v.IdentityPath)
}

func (v *MachineVM) removeSystemConnections() error {
	return machine.RemoveConnections(v.Name, fmt.Sprintf("%s-root", v.Name))
}

func (v *MachineVM) Set(_ string, opts machine.SetOptions) ([]error, error) {
	// If one setting fails to be applied, the others settings will not fail and still be applied.
	// The setting(s) that failed to be applied will have its errors returned in setErrors
	var setErrors []error

	v.lock.Lock()
	defer v.lock.Unlock()

	state, err := v.State(false)
	if err != nil {
		return setErrors, err
	}

	if state == machine.Running {
		suffix := ""
		if v.Name != machine.DefaultMachineName {
			suffix = " " + v.Name
		}
		return setErrors, fmt.Errorf("cannot change settings while the vm is running, run 'podman machine stop%s' first", suffix)
	}

	if opts.Rootful != nil && v.Rootful != *opts.Rootful {
		if err := v.setRootful(*opts.Rootful); err != nil {
			setErrors = append(setErrors, fmt.Errorf("failed to set rootful option: %w", err))
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
			setErrors = append(setErrors, fmt.Errorf("failed to resize disk: %w", err))
		} else {
			v.DiskSize = *opts.DiskSize
		}
	}

	if opts.USBs != nil {
		if usbConfigs, err := parseUSBs(*opts.USBs); err != nil {
			setErrors = append(setErrors, fmt.Errorf("failed to set usb: %w", err))
		} else {
			v.USBs = usbConfigs
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

// mountVolumesToVM iterates through the machine's volumes and mounts them to the
// machine
func (v *MachineVM) mountVolumesToVM(opts machine.StartOptions, name string) error {
	for _, mount := range v.Mounts {
		if !opts.Quiet {
			fmt.Printf("Mounting volume... %s:%s\n", mount.Source, mount.Target)
		}
		// create mountpoint directory if it doesn't exist
		// because / is immutable, we have to monkey around with permissions
		// if we dont mount in /home or /mnt
		args := []string{"-q", "--"}
		if !strings.HasPrefix(mount.Target, "/home") && !strings.HasPrefix(mount.Target, "/mnt") {
			args = append(args, "sudo", "chattr", "-i", "/", ";")
		}
		args = append(args, "sudo", "mkdir", "-p", mount.Target)
		if !strings.HasPrefix(mount.Target, "/home") && !strings.HasPrefix(mount.Target, "/mnt") {
			args = append(args, ";", "sudo", "chattr", "+i", "/", ";")
		}
		err := v.SSH(name, machine.SSHOptions{Args: args})
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
	return nil
}

// conductVMReadinessCheck checks to make sure the machine is in the proper state
// and that SSH is up and running
func (v *MachineVM) conductVMReadinessCheck(name string, maxBackoffs int, backoff time.Duration) (connected bool, sshError error, err error) {
	for i := 0; i < maxBackoffs; i++ {
		if i > 0 {
			time.Sleep(backoff)
			backoff *= 2
		}
		state, err := v.State(true)
		if err != nil {
			return false, nil, err
		}
		if state == machine.Running && v.isListening() {
			// Also make sure that SSH is up and running.  The
			// ready service's dependencies don't fully make sure
			// that clients can SSH into the machine immediately
			// after boot.
			//
			// CoreOS users have reported the same observation but
			// the underlying source of the issue remains unknown.
			if sshError = v.SSH(name, machine.SSHOptions{Args: []string{"true"}}); sshError != nil {
				logrus.Debugf("SSH readiness check for machine failed: %v", sshError)
				continue
			}
			connected = true
			break
		}
	}
	return
}

// runStartVMCommand executes the command to start the VM
func runStartVMCommand(cmd *exec.Cmd) error {
	err := cmd.Start()
	if err != nil {
		// check if qemu was not found
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		// look up qemu again maybe the path was changed, https://github.com/containers/podman/issues/13394
		cfg, err := config.Default()
		if err != nil {
			return err
		}
		qemuBinaryPath, err := cfg.FindHelperBinary(QemuCommand, true)
		if err != nil {
			return err
		}
		cmd.Path = qemuBinaryPath
		err = cmd.Start()
		if err != nil {
			return fmt.Errorf("unable to execute %q: %w", cmd, err)
		}
	}

	return nil
}

// qemuPid returns -1 or the PID of the running QEMU instance.
func (v *MachineVM) qemuPid() (int, error) {
	pidData, err := os.ReadFile(v.VMPidFilePath.GetPath())
	if err != nil {
		// The file may not yet exist on start or have already been
		// cleaned up after stop, so we need to be defensive.
		if errors.Is(err, os.ErrNotExist) {
			return -1, nil
		}
		return -1, err
	}
	if len(pidData) == 0 {
		return -1, nil
	}

	pid, err := strconv.Atoi(strings.TrimRight(string(pidData), "\n"))
	if err != nil {
		logrus.Warnf("Reading QEMU pidfile: %v", err)
		return -1, nil
	}
	return findProcess(pid)
}

// Start executes the qemu command line and forks it
func (v *MachineVM) Start(name string, opts machine.StartOptions) error {
	var (
		conn           net.Conn
		err            error
		qemuSocketConn net.Conn
	)

	defaultBackoff := 500 * time.Millisecond
	maxBackoffs := 6

	v.lock.Lock()
	defer v.lock.Unlock()

	state, err := v.State(false)
	if err != nil {
		return err
	}
	switch state {
	case machine.Starting:
		return fmt.Errorf("cannot start VM %q: starting state indicates that a previous start has failed: please stop and restart the VM", v.Name)
	case machine.Running:
		return fmt.Errorf("cannot start VM %q: %w", v.Name, machine.ErrVMAlreadyRunning)
	}

	// If QEMU is running already, something went wrong and we cannot
	// proceed.
	qemuPid, err := v.qemuPid()
	if err != nil {
		return err
	}
	if qemuPid != -1 {
		return fmt.Errorf("cannot start VM %q: another instance of %q is already running with process ID %d: please stop and restart the VM", v.Name, v.CmdLine[0], qemuPid)
	}

	v.Starting = true
	if err := v.writeConfig(); err != nil {
		return fmt.Errorf("writing JSON file: %w", err)
	}
	doneStarting := func() {
		v.Starting = false
		if err := v.writeConfig(); err != nil {
			logrus.Errorf("Writing JSON file: %v", err)
		}
	}
	defer doneStarting()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		_, ok := <-c
		if !ok {
			return
		}
		doneStarting()
		os.Exit(1)
	}()
	defer close(c)

	if v.isIncompatible() {
		logrus.Errorf("machine %q is incompatible with this release of podman and needs to be recreated, starting for recovery only", v.Name)
	}

	forwardSock, forwardState, err := v.startHostNetworking()
	if err != nil {
		return fmt.Errorf("unable to start host networking: %q", err)
	}

	rtPath, err := getRuntimeDir()
	if err != nil {
		return err
	}

	// If the temporary podman dir is not created, create it
	podmanTempDir := filepath.Join(rtPath, "podman")
	if _, err := os.Stat(podmanTempDir); errors.Is(err, fs.ErrNotExist) {
		if mkdirErr := os.MkdirAll(podmanTempDir, 0755); mkdirErr != nil {
			return err
		}
	}

	// If the qemusocketpath exists and the vm is off/down, we should rm
	// it before the dial as to avoid a segv
	if err := v.QMPMonitor.Address.Delete(); err != nil {
		return err
	}

	qemuSocketConn, err = machine.DialSocketWithBackoffs(maxBackoffs, defaultBackoff, v.QMPMonitor.Address.Path)
	if err != nil {
		return err
	}
	defer qemuSocketConn.Close()

	fd, err := qemuSocketConn.(*net.UnixConn).File()
	if err != nil {
		return err
	}
	defer fd.Close()

	dnr, dnw, err := machine.GetDevNullFiles()
	if err != nil {
		return err
	}
	defer dnr.Close()
	defer dnw.Close()

	attr := new(os.ProcAttr)
	files := []*os.File{dnr, dnw, dnw, fd}
	attr.Files = files
	cmdLine := v.CmdLine

	cmdLine.SetPropagatedHostEnvs()

	// Disable graphic window when not in debug mode
	// Done in start, so we're not suck with the debug level we used on init
	if !logrus.IsLevelEnabled(logrus.DebugLevel) {
		cmdLine.SetDisplay("none")
	}

	logrus.Debugf("qemu cmd: %v", cmdLine)

	stderrBuf := &bytes.Buffer{}

	// actually run the command that starts the virtual machine
	cmd := &exec.Cmd{
		Args:       cmdLine,
		Path:       cmdLine[0],
		Stdin:      dnr,
		Stdout:     dnw,
		Stderr:     stderrBuf,
		ExtraFiles: []*os.File{fd},
	}

	if err := runStartVMCommand(cmd); err != nil {
		return err
	}
	defer cmd.Process.Release() //nolint:errcheck

	if !opts.Quiet {
		fmt.Println("Waiting for VM ...")
	}

	conn, err = machine.DialSocketWithBackoffsAndProcCheck(maxBackoffs, defaultBackoff, v.ReadySocket.GetPath(), checkProcessStatus, "qemu", cmd.Process.Pid, stderrBuf)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return err
	}

	// update the podman/docker socket service if the host user has been modified at all (UID or Rootful)
	if v.HostUser.Modified {
		if machine.UpdatePodmanDockerSockService(v, name, v.UID, v.Rootful) == nil {
			// Reset modification state if there are no errors, otherwise ignore errors
			// which are already logged
			v.HostUser.Modified = false
			_ = v.writeConfig()
		}
	}

	if len(v.Mounts) == 0 {
		machine.WaitAPIAndPrintInfo(
			forwardState,
			v.Name,
			findClaimHelper(),
			forwardSock,
			opts.NoInfo,
			v.isIncompatible(),
			v.Rootful,
		)
		return nil
	}

	connected, sshError, err := v.conductVMReadinessCheck(name, maxBackoffs, defaultBackoff)
	if err != nil {
		return err
	}

	if !connected {
		msg := "machine did not transition into running state"
		if sshError != nil {
			return fmt.Errorf("%s: ssh error: %v", msg, sshError)
		}
		return errors.New(msg)
	}

	// mount the volumes to the VM
	if err := v.mountVolumesToVM(opts, name); err != nil {
		return err
	}

	machine.WaitAPIAndPrintInfo(
		forwardState,
		v.Name,
		findClaimHelper(),
		forwardSock,
		opts.NoInfo,
		v.isIncompatible(),
		v.Rootful,
	)
	return nil
}

// propagateHostEnv is here for providing the ability to propagate
// proxy and SSL settings (e.g. HTTP_PROXY and others) on a start
// and avoid a need of re-creating/re-initiating a VM
func propagateHostEnv(cmdLine QemuCmd) QemuCmd {
	varsToPropagate := make([]string, 0)

	for k, v := range machine.GetProxyVariables() {
		varsToPropagate = append(varsToPropagate, fmt.Sprintf("%s=%q", k, v))
	}

	if sslCertFile, ok := os.LookupEnv("SSL_CERT_FILE"); ok {
		pathInVM := filepath.Join(machine.UserCertsTargetPath, filepath.Base(sslCertFile))
		varsToPropagate = append(varsToPropagate, fmt.Sprintf("%s=%q", "SSL_CERT_FILE", pathInVM))
	}

	if _, ok := os.LookupEnv("SSL_CERT_DIR"); ok {
		varsToPropagate = append(varsToPropagate, fmt.Sprintf("%s=%q", "SSL_CERT_DIR", machine.UserCertsTargetPath))
	}

	if len(varsToPropagate) > 0 {
		prefix := "name=opt/com.coreos/environment,string="
		envVarsJoined := strings.Join(varsToPropagate, "|")
		fwCfgArg := prefix + base64.StdEncoding.EncodeToString([]byte(envVarsJoined))
		return append(cmdLine, "-fw_cfg", fwCfgArg)
	}

	return cmdLine
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
		if errors.Is(err, os.ErrNotExist) {
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

// waitForMachineToStop waits for the machine to stop running
func (v *MachineVM) waitForMachineToStop() error {
	fmt.Println("Waiting for VM to stop running...")
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
	// after the machine stops running it normally takes about 1 second for the
	// qemu VM to exit so we wait a bit to try to avoid issues
	time.Sleep(2 * time.Second)
	return nil
}

// ProxyPID retrieves the pid from the proxy pidfile
func (v *MachineVM) ProxyPID() (int, error) {
	if _, err := os.Stat(v.PidFilePath.Path); errors.Is(err, fs.ErrNotExist) {
		return -1, nil
	}
	proxyPidString, err := v.PidFilePath.Read()
	if err != nil {
		return -1, err
	}
	proxyPid, err := strconv.Atoi(string(proxyPidString))
	if err != nil {
		return -1, err
	}
	return proxyPid, nil
}

// cleanupVMProxyProcess kills the proxy process and removes the VM's pidfile
func (v *MachineVM) cleanupVMProxyProcess(proxyProc *os.Process) error {
	// Kill the process
	if err := proxyProc.Kill(); err != nil {
		return err
	}
	// Remove the pidfile
	if err := v.PidFilePath.Delete(); err != nil {
		return err
	}
	return nil
}

// VMPid retrieves the pid from the VM's pidfile
func (v *MachineVM) VMPid() (int, error) {
	vmPidString, err := v.VMPidFilePath.Read()
	if err != nil {
		return -1, err
	}
	vmPid, err := strconv.Atoi(strings.TrimSpace(string(vmPidString)))
	if err != nil {
		return -1, err
	}

	return vmPid, nil
}

// Stop uses the qmp monitor to call a system_powerdown
func (v *MachineVM) Stop(_ string, _ machine.StopOptions) error {
	v.lock.Lock()
	defer v.lock.Unlock()

	if err := v.update(); err != nil {
		return err
	}

	stopErr := v.stopLocked()

	// Make sure that the associated QEMU process gets killed in case it's
	// still running (#16054).
	qemuPid, err := v.qemuPid()
	if err != nil {
		if stopErr == nil {
			return err
		}
		return fmt.Errorf("%w: %w", stopErr, err)
	}

	if qemuPid == -1 {
		return stopErr
	}

	if err := sigKill(qemuPid); err != nil {
		if stopErr == nil {
			return err
		}
		return fmt.Errorf("%w: %w", stopErr, err)
	}

	return stopErr
}

// stopLocked stops the machine and expects the caller to hold the machine's lock.
func (v *MachineVM) stopLocked() error {
	// check if the qmp socket is there. if not, qemu instance is gone
	if _, err := os.Stat(v.QMPMonitor.Address.GetPath()); errors.Is(err, fs.ErrNotExist) {
		// Right now it is NOT an error to stop a stopped machine
		logrus.Debugf("QMP monitor socket %v does not exist", v.QMPMonitor.Address)
		// Fix incorrect starting state in case of crash during start
		if v.Starting {
			v.Starting = false
			if err := v.writeConfig(); err != nil {
				return fmt.Errorf("writing JSON file: %w", err)
			}
		}
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

	var disconnected bool
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

	proxyPid, err := v.ProxyPID()
	if err != nil || proxyPid < 0 {
		// may return nil if proxyPid == -1 because the pidfile does not exist
		return err
	}

	proxyProc, err := os.FindProcess(proxyPid)
	if proxyProc == nil && err != nil {
		return err
	}

	v.LastUp = time.Now()
	if err := v.writeConfig(); err != nil { // keep track of last up
		return err
	}

	if err := v.cleanupVMProxyProcess(proxyProc); err != nil {
		return err
	}

	// Remove socket
	if err := v.QMPMonitor.Address.Delete(); err != nil {
		return err
	}

	if err := qmpMonitor.Disconnect(); err != nil {
		// FIXME: this error should probably be returned
		return nil //nolint: nilerr
	}
	disconnected = true

	if err := v.ReadySocket.Delete(); err != nil {
		return err
	}

	if v.VMPidFilePath.GetPath() == "" {
		// no vm pid file path means it's probably a machine created before we
		// started using it, so we revert to the old way of waiting for the
		// machine to stop
		return v.waitForMachineToStop()
	}

	vmPid, err := v.VMPid()
	if err != nil {
		return err
	}

	fmt.Println("Waiting for VM to exit...")
	for isProcessAlive(vmPid) {
		time.Sleep(500 * time.Millisecond)
	}

	return nil
}

// NewQMPMonitor creates the monitor subsection of our vm
func NewQMPMonitor(network, name string, timeout time.Duration) (Monitor, error) {
	rtDir, err := getRuntimeDir()
	if err != nil {
		return Monitor{}, err
	}
	if isRootful() {
		rtDir = "/run"
	}
	rtDir = filepath.Join(rtDir, "podman")
	if _, err := os.Stat(rtDir); errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(rtDir, 0755); err != nil {
			return Monitor{}, err
		}
	}
	if timeout == 0 {
		timeout = defaultQMPTimeout
	}
	address, err := define.NewMachineFile(filepath.Join(rtDir, "qmp_"+name+".sock"), nil)
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

// collectFilesToDestroy retrieves the files that will be destroyed by `Remove`
func (v *MachineVM) collectFilesToDestroy(opts machine.RemoveOptions) ([]string, error) {
	files := []string{}
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
		return nil, err
	}
	if socketPath.Symlink != nil {
		files = append(files, *socketPath.Symlink)
	}
	files = append(files, socketPath.Path)
	files = append(files, v.archRemovalFiles()...)

	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return nil, err
	}
	files = append(files, filepath.Join(vmConfigDir, v.Name+".json"))

	return files, nil
}

// removeQMPMonitorSocketAndVMPidFile removes the VM pidfile, proxy pidfile,
// and QMP Monitor Socket
func (v *MachineVM) removeQMPMonitorSocketAndVMPidFile() {
	// remove socket and pid file if any: warn at low priority if things fail
	// Remove the pidfile
	if err := v.VMPidFilePath.Delete(); err != nil {
		logrus.Debugf("Error while removing VM pidfile: %v", err)
	}
	if err := v.PidFilePath.Delete(); err != nil {
		logrus.Debugf("Error while removing proxy pidfile: %v", err)
	}
	// Remove socket
	if err := v.QMPMonitor.Address.Delete(); err != nil {
		logrus.Debugf("Error while removing podman-machine-socket: %v", err)
	}
}

// Remove deletes all the files associated with a machine including ssh keys, the image itself
func (v *MachineVM) Remove(_ string, opts machine.RemoveOptions) (string, func() error, error) {
	var (
		files []string
	)

	v.lock.Lock()
	defer v.lock.Unlock()

	// cannot remove a running vm unless --force is used
	state, err := v.State(false)
	if err != nil {
		return "", nil, err
	}
	if state == machine.Running {
		if !opts.Force {
			return "", nil, &machine.ErrVMRunningCannotDestroyed{Name: v.Name}
		}
		err := v.stopLocked()
		if err != nil {
			return "", nil, err
		}
	}

	files, err = v.collectFilesToDestroy(opts)
	if err != nil {
		return "", nil, err
	}

	confirmationMessage := "\nThe following files will be deleted:\n\n"
	for _, msg := range files {
		confirmationMessage += msg + "\n"
	}

	v.removeQMPMonitorSocketAndVMPidFile()

	confirmationMessage += "\n"
	return confirmationMessage, func() error {
		machine.RemoveFilesAndConnections(files, v.Name, v.Name+"-root")
		return nil
	}, nil
}

func (v *MachineVM) State(bypass bool) (machine.Status, error) {
	// Check if qmp socket path exists
	if _, err := os.Stat(v.QMPMonitor.Address.GetPath()); errors.Is(err, fs.ErrNotExist) {
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
		// If an improper cleanup was done and the socketmonitor was not deleted,
		// it can appear as though the machine state is not stopped.  Check for ECONNREFUSED
		// almost assures us that the vm is stopped.
		if errors.Is(err, syscall.ECONNREFUSED) {
			return machine.Stopped, nil
		}
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
	// If there is a monitor, let's see if we can query state
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
		return fmt.Errorf("vm %q is not running", v.Name)
	}

	username := opts.Username
	if username == "" {
		username = v.RemoteUsername
	}

	return machine.CommonSSH(username, v.IdentityPath, v.Name, v.Port, opts.Args)
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

// startHostNetworking runs a binary on the host system that allows users
// to set up port forwarding to the podman virtual machine
func (v *MachineVM) startHostNetworking() (string, machine.APIForwardingState, error) {
	cfg, err := config.Default()
	if err != nil {
		return "", machine.NoForwarding, err
	}
	binary, err := cfg.FindHelperBinary(machine.ForwarderBinaryName, false)
	if err != nil {
		return "", machine.NoForwarding, err
	}

	cmd := gvproxy.NewGvproxyCommand()
	cmd.AddQemuSocket(fmt.Sprintf("unix://%s", v.QMPMonitor.Address.GetPath()))
	cmd.PidFile = v.PidFilePath.GetPath()
	cmd.SSHPort = v.Port

	var forwardSock string
	var state machine.APIForwardingState
	if !v.isIncompatible() {
		cmd, forwardSock, state = v.setupAPIForwarding(cmd)
	}

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		cmd.Debug = true
		logrus.Debug(cmd)
	}
	c := cmd.Cmd(binary)
	logrus.Debugf("gvproxy args: %v", c.Args)
	if err := c.Start(); err != nil {
		return "", 0, fmt.Errorf("unable to execute: %q: %w", cmd.ToCmdline(), err)
	}
	return forwardSock, state, nil
}

func (v *MachineVM) setupAPIForwarding(cmd gvproxy.GvproxyCommand) (gvproxy.GvproxyCommand, string, machine.APIForwardingState) {
	socket, err := v.forwardSocketPath()

	if err != nil {
		return cmd, "", machine.NoForwarding
	}

	destSock := fmt.Sprintf("/run/user/%d/podman/podman.sock", v.UID)

	forwardUser := v.RemoteUsername

	if v.Rootful {
		destSock = "/run/podman/podman.sock"
		forwardUser = "root"
	}

	cmd.AddForwardSock(socket.GetPath())
	cmd.AddForwardDest(destSock)
	cmd.AddForwardUser(forwardUser)
	cmd.AddForwardIdentity(v.IdentityPath)

	// The linking pattern is /var/run/docker.sock -> user global sock (link) -> machine sock (socket)
	// This allows the helper to only have to maintain one constant target to the user, which can be
	// repositioned without updating docker.sock.

	link, err := v.userGlobalSocketLink()
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

	return cmd, dockerSock, machine.DockerGlobal
}

func (v *MachineVM) isIncompatible() bool {
	return v.UID == -1
}

func (v *MachineVM) userGlobalSocketLink() (string, error) {
	path, err := machine.GetDataDir(machine.QemuVirt)
	if err != nil {
		logrus.Errorf("Resolving data dir: %s", err.Error())
		return "", err
	}
	// User global socket is located in parent directory of machine dirs (one per user)
	return filepath.Join(filepath.Dir(path), "podman.sock"), err
}

func (v *MachineVM) forwardSocketPath() (*define.VMFile, error) {
	sockName := "podman.sock"
	path, err := machine.GetDataDir(machine.QemuVirt)
	if err != nil {
		logrus.Errorf("Resolving data dir: %s", err.Error())
		return nil, err
	}
	return define.NewMachineFile(filepath.Join(path, sockName), &sockName)
}

func (v *MachineVM) setConfigPath() error {
	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return err
	}

	configPath, err := define.NewMachineFile(filepath.Join(vmConfigDir, v.Name)+".json", nil)
	if err != nil {
		return err
	}
	v.ConfigPath = *configPath
	return nil
}

func (v *MachineVM) setPIDSocket() error {
	rtPath, err := getRuntimeDir()
	if err != nil {
		return err
	}
	if isRootful() {
		rtPath = "/run"
	}
	socketDir := filepath.Join(rtPath, "podman")
	vmPidFileName := fmt.Sprintf("%s_vm.pid", v.Name)
	proxyPidFileName := fmt.Sprintf("%s_proxy.pid", v.Name)
	vmPidFilePath, err := define.NewMachineFile(filepath.Join(socketDir, vmPidFileName), &vmPidFileName)
	if err != nil {
		return err
	}
	proxyPidFilePath, err := define.NewMachineFile(filepath.Join(socketDir, proxyPidFileName), &proxyPidFileName)
	if err != nil {
		return err
	}
	v.VMPidFilePath = *vmPidFilePath
	v.PidFilePath = *proxyPidFilePath
	return nil
}

// Deprecated: getSocketandPid is being replaced by setPIDSocket and
// machinefiles.
func (v *MachineVM) getSocketandPid() (string, string, error) {
	rtPath, err := getRuntimeDir()
	if err != nil {
		return "", "", err
	}
	if isRootful() {
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

// update returns the content of the VM's
// configuration file in json
func (v *MachineVM) update() error {
	if err := v.setConfigPath(); err != nil {
		return err
	}
	b, err := v.ConfigPath.Read()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%v: %w", v.Name, machine.ErrNoSuchVM)
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
	return machine.WriteConfig(v.ConfigPath.Path, v)
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
	connInfo := new(machine.ConnectionConfig)
	podmanSocket, err := v.forwardSocketPath()
	if err != nil {
		return nil, err
	}
	connInfo.PodmanSocket = podmanSocket
	return &machine.InspectInfo{
		ConfigPath:         v.ConfigPath,
		ConnectionInfo:     *connInfo,
		Created:            v.Created,
		Image:              v.ImageConfig,
		LastUp:             v.LastUp,
		Name:               v.Name,
		Resources:          v.ResourceConfig,
		SSHConfig:          v.SSHConfig,
		State:              state,
		UserModeNetworking: true, // always true
		Rootful:            v.Rootful,
	}, nil
}

// resizeDisk increases the size of the machine's disk in GB.
func (v *MachineVM) resizeDisk(diskSize uint64, oldSize uint64) error {
	// Resize the disk image to input disk size
	// only if the virtualdisk size is less than
	// the given disk size
	if diskSize < oldSize {
		return fmt.Errorf("new disk size must be larger than current disk size: %vGB", oldSize)
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
		return fmt.Errorf("resizing image: %q", err)
	}

	return nil
}

func (v *MachineVM) setRootful(rootful bool) error {
	if err := machine.SetRootful(rootful, v.Name, v.Name+"-root"); err != nil {
		return err
	}

	v.HostUser.Modified = true
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

func isRootful() bool {
	// Rootless is not relevant on Windows. In the future rootless.IsRootless
	// could be switched to return true on Windows, and other codepaths migrated
	// for now will check additionally for valid os.Getuid

	return !rootless.IsRootless() && os.Getuid() != -1
}

func extractSourcePath(paths []string) string {
	return paths[0]
}

func extractMountOptions(paths []string) (bool, string) {
	readonly := false
	securityModel := "none"
	if len(paths) > 2 {
		options := paths[2]
		volopts := strings.Split(options, ",")
		for _, o := range volopts {
			switch {
			case o == "rw":
				readonly = false
			case o == "ro":
				readonly = true
			case strings.HasPrefix(o, "security_model="):
				securityModel = strings.Split(o, "=")[1]
			default:
				fmt.Printf("Unknown option: %s\n", o)
			}
		}
	}
	return readonly, securityModel
}
