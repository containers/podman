//go:build (amd64 && !windows) || !arm64
// +build amd64,!windows !arm64

package vbox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/storage/pkg/homedir"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	vboxArtifact = FcosArtifact{
		Artifact: "metal",
		Format:   "raw.xz",
	}
)

func (vbm *MachineVM) Init(opts machine.InitOptions) (bool, error) {
	var (
		key string
	)
	sshDir := filepath.Join(homedir.Get(), ".ssh")
	// GetConfDir creates the directory so no need to check for
	// its existence
	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return false, err
	}
	jsonFile := filepath.Join(vmConfigDir, vbm.Name) + ".json"
	vbm.IdentityPath = filepath.Join(sshDir, vbm.Name)

	switch opts.ImagePath {
	case "testing", "next", "stable", "":
		errMsg := `sorry, but Fedora CoreOS does not provide images for VirtualBox
There is an unofficial image, which could be used with no any sort of warranty:
Image: https://objectstorage.eu-amsterdam-1.oraclecloud.com/n/axsgbe2wuvbg/b/unofficial-fcos-image-provider-vbox/o/fedora-coreos-35.20220213.2.0-metal.x86_64.raw.xz
SHA256sum: https://objectstorage.eu-amsterdam-1.oraclecloud.com/n/axsgbe2wuvbg/b/unofficial-fcos-image-provider-vbox/o/fedora-coreos-35.20220213.2.0-metal.x86_64.raw.xz.sha256sum`
		return false, errors.Errorf(errMsg)
	default:
		// Get user-defined image
		vboxArtifact.Stream = "user-defined"
		vbm.ImageStream = vboxArtifact.Stream
		dd, err := machine.NewGenericDownloader(vmtype, vbm.Name, opts.ImagePath)

		if err != nil {
			return false, err
		}
		vbm.ImagePath = dd.Get().LocalUncompressedFile
		vbm.BaseFolder = filepath.Dir(vbm.ImagePath)
		vbm.VIDPath = strings.Replace(vbm.ImagePath, ".raw", ".vdi", -1)
		if err := machine.DownloadImage(dd); err != nil {
			return false, err
		}
	}

	// Creating VBox VM and configuring
	stages := make([][]string, 0)
	// Create and register VBos VM
	stages = append(stages, []string{"createvm",
		"--name", vbm.Name,
		"--basefolder", vbm.BaseFolder,
		"--ostype", vbm.VBOSType,
		"--register"})
	// Convert raw disk to VDI format
	stages = append(stages, []string{"convertdd", vbm.ImagePath, vbm.VIDPath, "--format", "VDI"})
	// Convert resize disk
	stages = append(stages, []string{"modifymedium", "disk", vbm.VIDPath, "--compact", "--resize", strconv.Itoa(int(vbm.DiskSize * 1024))})
	// Create a storage controller
	stages = append(stages, []string{"storagectl", vbm.Name,
		"--name", "sata-controller-" + vbm.Name,
		"--add", "sata",
		"--controller", "IntelAhci",
		"--portcount", "30",
		"--bootable", "on"})
	// Attache disk to VM
	stages = append(stages, []string{"storageattach", vbm.Name,
		"--storagectl", "sata-controller-" + vbm.Name,
		"--port", "0",
		"--device", "0",
		"--type", "hdd",
		"--medium", vbm.VIDPath})
	stages = append(stages, []string{"modifyvm", vbm.Name,
		"--boot1", "disk", "--boot2", "none", "--boot3", "none", "--boot4", "none",
		"--cpus", strconv.Itoa(int(vbm.CPUs)),
		"--ioapic", "on",
		"--memory", strconv.Itoa(int(vbm.Memory)),
		"--vram", "20",
		"--graphicscontroller", "vmsvga",
		"--rtcuseutc", "on",
		"--nic1", "nat",
		"--natpf1", fmt.Sprintf("SSH,tcp,127.0.0.1,%d,10.0.2.15,22", vbm.Port)})
	fmt.Println("Creating VBox VM...")
	for i, cmd := range stages {
		step := exec.Command(vbm.VBoxManageExecPath, cmd...)
		step.Stderr = os.Stderr
		if err := step.Run(); err != nil {
			return false, errors.Errorf("error on stage: %d, : %q", i, err)
		}
	}
	// Remove raw image because it's no needed more
	os.Remove(vbm.ImagePath)
	vbm.ImagePath = "[REMOVED]" + vbm.ImagePath

	fi, err := os.Stat(vbm.VIDPath)
	if err != nil {
		return false, fmt.Errorf("VDI image is not exist: %q", err)
	}
	vbm.CreatingTime = fi.ModTime()

	// This kind of stinks but no other way around this r/n
	if len(opts.IgnitionPath) < 1 {
		uri := machine.SSHRemoteConnection.MakeSSHURL("localhost", "/run/user/1000/podman/podman.sock", strconv.Itoa(vbm.Port), vbm.RemoteUsername)
		if err := machine.AddConnection(&uri, vbm.Name, filepath.Join(sshDir, vbm.Name), opts.IsDefault); err != nil {
			return false, err
		}

		uriRoot := machine.SSHRemoteConnection.MakeSSHURL("localhost", "/run/podman/podman.sock", strconv.Itoa(vbm.Port), "root")
		if err := machine.AddConnection(&uriRoot, vbm.Name+"-root", filepath.Join(sshDir, vbm.Name), opts.IsDefault); err != nil {
			return false, err
		}
	} else {
		fmt.Println("An ignition path was provided.  No SSH connection was added to Podman")
	}

	// Write the JSON file
	b, err := json.MarshalIndent(vbm, "", " ")
	if err != nil {
		return false, err
	}
	if err := ioutil.WriteFile(jsonFile, b, 0644); err != nil {
		return false, err
	}

	// User has provided ignition file so keygen
	// will be skipped.
	if len(opts.IgnitionPath) < 1 {
		key, err = machine.CreateSSHKeys(vbm.IdentityPath)
		if err != nil {
			return false, err
		}
	}

	// If the user provides an ignition file, we need to
	// copy it into the conf dir
	if len(opts.IgnitionPath) > 0 {
		inputIgnition, err := ioutil.ReadFile(opts.IgnitionPath)
		if err != nil {
			return false, err
		}
		return false, ioutil.WriteFile(vbm.IgnitionFilePath, inputIgnition, 0644)
	}
	// Write the ignition file
	ign := machine.DynamicIgnition{
		Name:      opts.Username,
		Key:       key,
		VMName:    vbm.Name,
		TimeZone:  opts.TimeZone,
		WritePath: vbm.IgnitionFilePath,
	}
	err = machine.NewIgnitionFile(ign)
	if err != nil {
		return false, err
	}
	ignCont, err := ioutil.ReadFile(vbm.IgnitionFilePath)
	if err != nil {
		return false, err
	}
	ignProp := exec.Command(vbm.VBoxManageExecPath, "guestproperty", "set", vbm.Name, "/Ignition/Config", string(ignCont))
	ignProp.Stderr = os.Stderr
	if err := ignProp.Run(); err != nil {
		return false, errors.Errorf("error on setting guestproperty to VM: %s, error: %q", vbm.Name, err)
	}
	return err == nil, err
}

func (vbm *MachineVM) Remove(name string, opts machine.RemoveOptions) (string, func() error, error) {
	var (
		files []string
	)

	running, err := vbm.isRunning()
	if err != nil {
		return "", nil, err
	}
	if running {
		return "", nil, errors.Errorf("running vm %q cannot be destroyed", vbm.Name)
	}

	// Collect all the files that need to be destroyed
	if !opts.SaveKeys {
		files = append(files, vbm.IdentityPath, vbm.IdentityPath+".pub")
	}
	if !opts.SaveIgnition {
		files = append(files, vbm.IgnitionFilePath)
	}
	if err := machine.RemoveConnection(vbm.Name); err != nil {
		logrus.Error(err)
	}
	if err := machine.RemoveConnection(vbm.Name + "-root"); err != nil {
		logrus.Error(err)
	}

	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return "", nil, err
	}
	files = append(files, filepath.Join(vmConfigDir, vbm.Name+".json"))
	confirmationMessage := "\nThe following files will be deleted:\n\n"
	for _, msg := range files {
		confirmationMessage += msg + "\n"
	}

	confirmationMessage += "\n"
	return confirmationMessage, func() error {
		cmdLine := []string{"unregistervm", vbm.Name}
		if !opts.SaveImage {
			cmdLine = append(cmdLine, "--delete")
		}
		cmd := exec.Command(vbm.VBoxManageExecPath, cmdLine...)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			logrus.Error(err)
		}
		for _, f := range files {
			if err := os.Remove(f); err != nil {
				logrus.Error(err)
			}
		}
		return nil
	}, nil
}

func (vbm *MachineVM) SSH(name string, opts machine.SSHOptions) error {
	running, err := vbm.isRunning()
	if err != nil {
		return err
	}
	if !running {
		return errors.Errorf("vm %q is not running.", vbm.Name)
	}

	username := opts.Username
	if username == "" {
		username = vbm.RemoteUsername
	}

	sshDestination := username + "@localhost"
	port := strconv.Itoa(vbm.Port)

	args := []string{"-i", vbm.IdentityPath, "-p", port, sshDestination, "-o", "UserKnownHostsFile /dev/null", "-o", "StrictHostKeyChecking no"}
	if len(opts.Args) > 0 {
		args = append(args, opts.Args...)
	} else {
		fmt.Printf("Connecting to vm %s. To close connection, use `~.` or `exit`\n", vbm.Name)
	}

	cmd := exec.Command("ssh", args...)
	logrus.Debugf("Executing: ssh %v\n", args)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

func (vbm *MachineVM) Start(name string, opts machine.StartOptions) error {
	var cmd []string

	vmState, err := vbm.getState()
	if err != nil {
		return err
	}

	fi, err := os.Stat(vbm.VIDPath)
	if err != nil {
		return fmt.Errorf("VDI image is not exist: %q", err)
	}
	isFirstTime := vbm.CreatingTime == fi.ModTime()

	switch vmState {
	case "running":
		return fmt.Errorf("VM \"%s\" is already started", vbm.Name)
	case "paused":
		cmd = []string{"controlvm", vbm.Name, "resume", "--type", "headless"}
		fmt.Printf("Resuming \"%s\" VM ...\n", vbm.Name)
	case "saved", "poweroff", "aborted":
		cmd = []string{"startvm", vbm.Name, "--type", "headless"}
	default:
		return fmt.Errorf("VM \"%s\" has unknown state", vbm.Name)
	}
	cmdExec := exec.Command(vbm.VBoxManageExecPath, cmd...)
	cmdExec.Stderr = os.Stderr
	if err := cmdExec.Run(); err != nil {
		return err
	}

	for try := 10; try > 0 && vmState != "running"; try-- {
		vmState, _ = vbm.getState()
		time.Sleep(time.Second * 5)
	}
	if vmState != "running" {
		return fmt.Errorf("somthing went wrong with \"%s\", its state is \"%s\"", vbm.Name, vmState)
	}

	// Let's allow Ignition do its work
	if isFirstTime {
		fmt.Printf("It's the first start of VM, Please, wait for a while to allow Ignition to finalize the configuration of it.")
		for try := 13; try > 0; try-- {
			fmt.Printf(".")
			time.Sleep(time.Second * 2)
		}
		fmt.Println()
	}

	if !vbm.Rootful {
		fmt.Printf("\nThis machine is currently configured in rootless mode. If your containers\n")
		fmt.Printf("require root permissions (e.g. ports < 1024), or if you run into compatibility\n")
		fmt.Printf("issues with non-podman clients, you can switch using the following command: \n")

		suffix := ""
		if name != DefaultMachineName {
			suffix = " " + name
		}
		fmt.Printf("\n\tpodman machine set --rootful%s\n\n", suffix)
	}
	return nil
}

func (vbm *MachineVM) Stop(name string, opts machine.StopOptions) error {
	vmState, err := vbm.getState()
	if err != nil {
		return err
	}

	var cmd []string

	switch vmState {
	case "paused", "running":
		cmd = []string{"controlvm", vbm.Name, "poweroff"}
	case "saved", "poweroff", "aborted":
		return fmt.Errorf("VM \"%s\" is already stoped", vbm.Name)
	default:
		return fmt.Errorf("VM \"%s\" has unknown state", vbm.Name)
	}
	cmdExec := exec.Command(vbm.VBoxManageExecPath, cmd...)
	cmdExec.Stderr = os.Stderr
	if err := cmdExec.Run(); err != nil {
		return err
	}
	for try := 10; try > 0 && vmState != "poweroff"; try-- {
		vmState, _ = vbm.getState()
		time.Sleep(time.Second * 5)
	}
	if vmState != "poweroff" {
		return fmt.Errorf("somthing went wrong with \"%s\", its state is \"%s\"", vbm.Name, vmState)
	}
	return nil
}

func (vbm *MachineVM) Set(name string, opts machine.SetOptions) error {
	if vbm.Rootful == opts.Rootful {
		return nil
	}

	changeCon, err := machine.AnyConnectionDefault(vbm.Name, vbm.Name+"-root")
	if err != nil {
		return err
	}

	if changeCon {
		newDefault := vbm.Name
		if opts.Rootful {
			newDefault += "-root"
		}
		if err := machine.ChangeDefault(newDefault); err != nil {
			return err
		}
	}

	vbm.Rootful = opts.Rootful
	return vbm.writeConfig()
}

func (vbm *MachineVM) getState() (string, error) {
	var state string
	sCmd := exec.Command(vbm.VBoxManageExecPath, "showvminfo", vbm.Name, "--machinereadable")
	var outb bytes.Buffer
	sCmd.Stdout = &outb
	sCmd.Stderr = os.Stderr
	if err := sCmd.Run(); err != nil {
		return state, errors.Errorf("error on getting vm state: %q", err)
	}

	re := regexp.MustCompile(`(?m)^VMState="(\w+)"`)
	groups := re.FindStringSubmatch(outb.String())
	if len(groups) < 1 {
		return state, fmt.Errorf("error, can't get a state of vm %s", vbm.Name)
	}
	state = groups[1]
	return state, nil
}

func (vbm *MachineVM) isRunning() (bool, error) {
	vmState, err := vbm.getState()
	if err != nil {
		return false, err
	}
	if vmState == "running" {
		return true, nil
	}
	return false, nil
}

func (vbm *MachineVM) writeConfig() error {
	// GetConfDir creates the directory so no need to check for
	// its existence
	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return err
	}

	jsonFile := filepath.Join(vmConfigDir, vbm.Name) + ".json"
	// Write the JSON file
	b, err := json.MarshalIndent(vbm, "", " ")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(jsonFile, b, 0644); err != nil {
		return err
	}

	return nil
}
