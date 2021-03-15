package qemu

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/containers/podman/v3/pkg/machine"
	"github.com/containers/podman/v3/pkg/specgen"
	"github.com/containers/storage/pkg/homedir"
	"github.com/digitalocean/go-qemu/qmp"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	// vmtype refers to qemu (vs libvirt, krun, etc)
	vmtype = "qemu"
	// qemuCommon are the common command line arguments between the arches
	//qemuCommon = []string{"-cpu", "host", "-qmp", "unix://tmp/qmp.sock,server,nowait"}
	//qemuCommon  = []string{"-cpu", "host", "-qmp", "tcp:localhost:4444,server,nowait"}
)

// NewMachine creates an instance of a virtual machine based on the qemu
// virtualization.
func NewMachine(opts machine.CreateOptions) (machine.VM, error) {
	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return nil, err
	}
	vm := new(MachineVM)
	if len(opts.Name) > 0 {
		vm.Name = opts.Name
	}
	vm.IgnitionFilePath = opts.IgnitionPath
	// If no ignitionfilepath was provided, use defaults
	if len(vm.IgnitionFilePath) < 1 {
		ignitionFile := filepath.Join(vmConfigDir, vm.Name+".ign")
		vm.IgnitionFilePath = ignitionFile
	}

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
	port, err := specgen.GetRandomPort()
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
	// TODO
	// Add ignition file
	cmd = append(cmd, []string{"-fw_cfg", "name=opt/com.coreos/config,file=" + vm.IgnitionFilePath}...)
	// Add qmp socket
	monitor, err := NewQMPMonitor("unix", vm.Name, defaultQMPTimeout)
	if err != nil {
		return nil, err
	}
	vm.QMPMonitor = monitor
	cmd = append(cmd, []string{"-qmp", monitor.Network + ":/" + monitor.Address + ",server,nowait"}...)

	// Add network
	cmd = append(cmd, "-nic", "user,model=virtio,hostfwd=tcp::"+strconv.Itoa(vm.Port)+"-:22")
	vm.CmdLine = cmd
	fmt.Println("///")
	return vm, nil
}

// LoadByName reads a json file that describes a known qemu vm
// and returns a vm instance
func LoadVMByName(name string) (machine.VM, error) {
	// TODO need to define an error relating to ErrMachineNotFound
	vm := new(MachineVM)
	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return nil, err
	}
	b, err := ioutil.ReadFile(filepath.Join(vmConfigDir, name+".json"))
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(b, vm)
	logrus.Debug(vm.CmdLine)
	return vm, err
}

// Create writes the json configuration file to the filesystem for
// other verbs (start, stop)
func (v *MachineVM) Create(opts machine.CreateOptions) error {
	sshDir := filepath.Join(homedir.Get(), ".ssh")
	// GetConfDir creates the directory so no need to check for
	// its existence
	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return err
	}
	jsonFile := filepath.Join(vmConfigDir, v.Name) + ".json"
	v.IdentityPath = filepath.Join(sshDir, v.Name)

	dd, err := machine.NewFcosDownloader(vmtype, v.Name)
	if err != nil {
		return err
	}

	v.ImagePath = dd.Get().LocalUncompressedFile
	if err := dd.DownloadImage(); err != nil {
		return err
	}
	// Add arch specific options including image location
	v.CmdLine = append(v.CmdLine, v.addArchOptions()...)

	// Add location of bootable image
	v.CmdLine = append(v.CmdLine, "-drive", "if=virtio,file="+v.ImagePath)
	// This kind of stinks but no other way around this r/n
	uri := machine.SSHRemoteConnection.MakeSSHURL("localhost", "/run/user/1000/podman/podman.sock", strconv.Itoa(v.Port), v.RemoteUsername)
	if err := machine.AddConnection(&uri, v.Name, filepath.Join(sshDir, v.Name), opts.IsDefault); err != nil {
		return err
	}
	// Write the JSON file
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(jsonFile, b, 0644); err != nil {
		return err
	}
	key, err := machine.CreateSSHKeys(v.IdentityPath)
	if err != nil {
		return err
	}
	// Run arch specific things that need to be done
	if err := v.prepare(); err != nil {
		return err
	}
	// Write the ignition file
	return machine.NewIgnitionFile(opts.Username, key, v.IgnitionFilePath)
}

// Start executes the qemu command line and forks it
func (v *MachineVM) Start(name string, _ machine.StartOptions) error {
	var (
		err error
	)
	attr := new(os.ProcAttr)
	files := []*os.File{os.Stdin, os.Stdout, os.Stderr}
	attr.Files = files
	fmt.Print(v.CmdLine)
	_, err = os.StartProcess(v.CmdLine[0], v.CmdLine, attr)
	return err
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
	_, err = qmpMonitor.Run(input)
	return err
}

// NewQMPMonitor creates the monitor subsection of our vm
func NewQMPMonitor(network, name string, timeout time.Duration) (Monitor, error) {
	rtDir, err := getDataDir()
	if err != nil {
		return Monitor{}, err
	}
	if timeout == 0 {
		timeout = defaultQMPTimeout
	}
	monitor := Monitor{
		Network: network,
		Address: filepath.Join(rtDir, "podman", "qmp_"+name+".sock"),
		Timeout: timeout,
	}
	return monitor, nil
}

func (v *MachineVM) Destroy(name string, opts machine.DestroyOptions) (string, func() error, error) {
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

	sshDestination := v.RemoteUsername + "@localhost"
	port := strconv.Itoa(v.Port)

	fmt.Printf("Connecting to vm %s. To close connection, use `~.` or `exit`\n", v.Name)

	cmd := exec.Command("ssh", "-i", v.IdentityPath, "-p", port, sshDestination)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}
