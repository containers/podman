//go:build windows

package hyperv

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Microsoft/go-winio"
	"github.com/containers/common/pkg/strongunits"
	gvproxy "github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/containers/libhvee/pkg/hypervctl"
	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/env"
	"github.com/containers/podman/v5/pkg/machine/hyperv/vsock"
	"github.com/containers/podman/v5/pkg/machine/ignition"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"github.com/containers/podman/v5/pkg/systemd/parser"
	"github.com/sirupsen/logrus"
)

type HyperVStubber struct {
	vmconfigs.HyperVConfig
}

func (h HyperVStubber) UserModeNetworkEnabled(mc *vmconfigs.MachineConfig) bool {
	return true
}

func (h HyperVStubber) UseProviderNetworkSetup() bool {
	return false
}

func (h HyperVStubber) RequireExclusiveActive() bool {
	return true
}

func (h HyperVStubber) CreateVM(opts define.CreateVMOpts, mc *vmconfigs.MachineConfig, builder *ignition.IgnitionBuilder) error {
	var (
		err error
	)
	callbackFuncs := machine.CleanUp()
	defer callbackFuncs.CleanIfErr(&err)
	go callbackFuncs.CleanOnSignal()

	hwConfig := hypervctl.HardwareConfig{
		CPUs:     uint16(mc.Resources.CPUs),
		DiskPath: mc.ImagePath.GetPath(),
		DiskSize: uint64(mc.Resources.DiskSize),
		Memory:   uint64(mc.Resources.Memory),
	}

	networkHVSock, err := vsock.NewHVSockRegistryEntry(mc.Name, vsock.Network)
	if err != nil {
		return err
	}

	mc.HyperVHypervisor.NetworkVSock = *networkHVSock

	// Add vsock port numbers to mounts
	err = createShares(mc)
	if err != nil {
		return err
	}

	removeShareCallBack := func() error {
		return removeShares(mc)
	}
	callbackFuncs.Add(removeShareCallBack)

	removeRegistrySockets := func() error {
		removeNetworkAndReadySocketsFromRegistry(mc)
		return nil
	}
	callbackFuncs.Add(removeRegistrySockets)

	netUnitFile, err := createNetworkUnit(mc.HyperVHypervisor.NetworkVSock.Port)
	if err != nil {
		return err
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

	vmm := hypervctl.NewVirtualMachineManager()
	err = vmm.NewVirtualMachine(mc.Name, &hwConfig)
	if err != nil {
		return err
	}

	vmRemoveCallback := func() error {
		vm, err := vmm.GetMachine(mc.Name)
		if err != nil {
			return err
		}
		return vm.Remove("")
	}

	callbackFuncs.Add(vmRemoveCallback)
	err = resizeDisk(mc.Resources.DiskSize, mc.ImagePath)
	return err
}

func (h HyperVStubber) Exists(name string) (bool, error) {
	vmm := hypervctl.NewVirtualMachineManager()
	exists, _, err := vmm.GetMachineExists(name)
	return exists, err
}

func (h HyperVStubber) MountType() vmconfigs.VolumeMountType {
	return vmconfigs.NineP
}

func (h HyperVStubber) MountVolumesToVM(mc *vmconfigs.MachineConfig, quiet bool) error {
	return nil
}

func (h HyperVStubber) Remove(mc *vmconfigs.MachineConfig) ([]string, func() error, error) {
	_, vm, err := GetVMFromMC(mc)
	if err != nil {
		return nil, nil, err
	}

	rmFunc := func() error {
		// Tear down vsocks
		removeNetworkAndReadySocketsFromRegistry(mc)

		// Remove ignition registry entries - not a fatal error
		// for vm removal
		// TODO we could improve this by recommending an action be done
		if err := removeIgnitionFromRegistry(vm); err != nil {
			logrus.Errorf("unable to remove ignition registry entries: %q", err)
		}

		// disk path removal is done by generic remove
		return vm.Remove("")
	}
	return []string{}, rmFunc, nil
}

func (h HyperVStubber) RemoveAndCleanMachines(_ *define.MachineDirs) error {
	return nil
}

func (h HyperVStubber) StartNetworking(mc *vmconfigs.MachineConfig, cmd *gvproxy.GvproxyCommand) error {
	cmd.AddEndpoint(fmt.Sprintf("vsock://%s", mc.HyperVHypervisor.NetworkVSock.KeyName))
	return nil
}

func (h HyperVStubber) StartVM(mc *vmconfigs.MachineConfig) (func() error, func() error, error) {
	var (
		err error
	)

	_, vm, err := GetVMFromMC(mc)
	if err != nil {
		return nil, nil, err
	}

	callbackFuncs := machine.CleanUp()
	defer callbackFuncs.CleanIfErr(&err)
	go callbackFuncs.CleanOnSignal()

	firstBoot, err := mc.IsFirstBoot()
	if err != nil {
		return nil, nil, err
	}

	if firstBoot {
		// Add ignition entries to windows registry
		// for first boot only
		if err := readAndSplitIgnition(mc, vm); err != nil {
			return nil, nil, err
		}

		// this is added because if the machine does not start
		// properly on first boot, the next boot will be considered
		// the first boot again and the addition of the ignition
		// entries might fail?
		//
		// the downside is that if the start fails and then a rm
		// is run, it will puke error messages about the ignition.
		//
		// TODO detect if ignition was run from a failed boot earlier
		// and skip.  Maybe this could be done with checking a k/v
		// pair
		rmIgnCallbackFunc := func() error {
			return removeIgnitionFromRegistry(vm)
		}
		callbackFuncs.Add(rmIgnCallbackFunc)
	}

	waitReady, listener, err := mc.HyperVHypervisor.ReadyVsock.ListenSetupWait()
	if err != nil {
		return nil, nil, err
	}

	err = vm.Start()
	if err != nil {
		// cleanup the pending listener
		_ = listener.Close()
		return nil, nil, err
	}

	startCallback := func() error {
		return vm.Stop()
	}
	callbackFuncs.Add(startCallback)

	return nil, waitReady, err
}

// State is returns the state as a define.status.  for hyperv, state differs from others because
// state is determined by the VM itself.  normally this can be done with vm.State() and a conversion
// but doing here as well.  this requires a little more interaction with the hypervisor
func (h HyperVStubber) State(mc *vmconfigs.MachineConfig, bypass bool) (define.Status, error) {
	_, vm, err := GetVMFromMC(mc)
	if err != nil {
		return define.Unknown, err
	}
	return stateConversion(vm.State())
}

func (h HyperVStubber) StopVM(mc *vmconfigs.MachineConfig, hardStop bool) error {
	vmm := hypervctl.NewVirtualMachineManager()
	vm, err := vmm.GetMachine(mc.Name)
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

	if hardStop {
		return vm.StopWithForce()
	}
	return vm.Stop()
}

// TODO should this be plumbed higher into the code stack?
func (h HyperVStubber) StopHostNetworking(mc *vmconfigs.MachineConfig, vmType define.VMType) error {
	err := machine.StopWinProxy(mc.Name, vmType)
	// in podman 4, this was a "soft" error; keeping behavior as such
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not stop API forwarding service (win-sshproxy.exe): %s\n", err.Error())
	}

	return nil
}

func (h HyperVStubber) VMType() define.VMType {
	return define.HyperVVirt
}

func GetVMFromMC(mc *vmconfigs.MachineConfig) (*hypervctl.VirtualMachineManager, *hypervctl.VirtualMachine, error) {
	vmm := hypervctl.NewVirtualMachineManager()
	vm, err := vmm.GetMachine(mc.Name)
	return vmm, vm, err
}

func stateConversion(s hypervctl.EnabledState) (define.Status, error) {
	switch s {
	case hypervctl.Enabled:
		return define.Running, nil
	case hypervctl.Disabled:
		return define.Stopped, nil
	case hypervctl.Starting:
		return define.Starting, nil
	}
	return define.Unknown, fmt.Errorf("unknown state: %q", s.String())
}

func (h HyperVStubber) SetProviderAttrs(mc *vmconfigs.MachineConfig, opts define.SetOptions) error {
	var (
		cpuChanged, memoryChanged bool
	)

	_, vm, err := GetVMFromMC(mc)
	if err != nil {
		return err
	}

	if vm.State() != hypervctl.Disabled {
		return errors.New("unable to change settings unless vm is stopped")
	}

	if opts.Rootful != nil && mc.HostUser.Rootful != *opts.Rootful {
		if err := mc.SetRootful(*opts.Rootful); err != nil {
			return err
		}
	}

	if opts.DiskSize != nil {
		if err := resizeDisk(*opts.DiskSize, mc.ImagePath); err != nil {
			return err
		}
	}
	if opts.CPUs != nil {
		cpuChanged = true
	}
	if opts.Memory != nil {
		memoryChanged = true
	}

	if cpuChanged || memoryChanged {
		err := vm.UpdateProcessorMemSettings(func(ps *hypervctl.ProcessorSettings) {
			if cpuChanged {
				ps.VirtualQuantity = *opts.CPUs
			}
		}, func(ms *hypervctl.MemorySettings) {
			if memoryChanged {
				ms.DynamicMemoryEnabled = false
				mem := uint64(*opts.Memory)
				ms.VirtualQuantity = mem
				ms.Limit = mem
				ms.Reservation = mem
			}
		})
		if err != nil {
			return fmt.Errorf("setting CPU and Memory for VM: %w", err)
		}
	}

	if opts.USBs != nil {
		return fmt.Errorf("changing USBs not supported for hyperv machines")
	}

	return nil
}

func (h HyperVStubber) PrepareIgnition(mc *vmconfigs.MachineConfig, ignBuilder *ignition.IgnitionBuilder) (*ignition.ReadyUnitOpts, error) {
	// HyperV is different because it has to know some ignition details before creating the VM.  It cannot
	// simply be derived. So we create the HyperVConfig here.
	mc.HyperVHypervisor = new(vmconfigs.HyperVConfig)
	var ignOpts ignition.ReadyUnitOpts
	readySock, err := vsock.NewHVSockRegistryEntry(mc.Name, vsock.Events)
	if err != nil {
		return nil, err
	}

	// TODO Stopped here ... fails bc mc.Hypervisor is nil ... this can be nil checked prior and created
	// however the same will have to be done in create
	mc.HyperVHypervisor.ReadyVsock = *readySock
	ignOpts.Port = readySock.Port
	return &ignOpts, nil
}

func (h HyperVStubber) PostStartNetworking(mc *vmconfigs.MachineConfig, noInfo bool) error {
	var (
		err        error
		executable string
	)
	callbackFuncs := machine.CleanUp()
	defer callbackFuncs.CleanIfErr(&err)
	go callbackFuncs.CleanOnSignal()

	if len(mc.Mounts) == 0 {
		return nil
	}

	var (
		dirs       *define.MachineDirs
		gvproxyPID int
	)
	dirs, err = env.GetMachineDirs(h.VMType())
	if err != nil {
		return err
	}
	// GvProxy PID file path is now derived
	gvproxyPIDFile, err := dirs.RuntimeDir.AppendToNewVMFile("gvproxy.pid", nil)
	if err != nil {
		return err
	}
	gvproxyPID, err = gvproxyPIDFile.ReadPIDFrom()
	if err != nil {
		return err
	}

	executable, err = os.Executable()
	if err != nil {
		return err
	}
	// Start the 9p server in the background
	p9ServerArgs := []string{}
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		p9ServerArgs = append(p9ServerArgs, "--log-level=debug")
	}
	p9ServerArgs = append(p9ServerArgs, "machine", "server9p")

	for _, mount := range mc.Mounts {
		if mount.VSockNumber == nil {
			return fmt.Errorf("mount %s has no vsock port defined", mount.Source)
		}
		p9ServerArgs = append(p9ServerArgs, "--serve", fmt.Sprintf("%s:%s", mount.Source, winio.VsockServiceID(uint32(*mount.VSockNumber)).String()))
	}
	p9ServerArgs = append(p9ServerArgs, fmt.Sprintf("%d", gvproxyPID))

	logrus.Debugf("Going to start 9p server using command: %s %v", executable, p9ServerArgs)

	fsCmd := exec.Command(executable, p9ServerArgs...)

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		err = logCommandToFile(fsCmd, "podman-machine-server9.log")
		if err != nil {
			return err
		}
	}

	err = fsCmd.Start()
	if err != nil {
		return fmt.Errorf("unable to start 9p server: %v", err)
	}
	logrus.Infof("Started podman 9p server as PID %d", fsCmd.Process.Pid)

	// Note: No callback is needed to stop the 9p server, because it will stop when
	// gvproxy stops

	// Finalize starting shares after we are confident gvproxy is still alive.
	err = startShares(mc)
	return err
}

func (h HyperVStubber) UpdateSSHPort(mc *vmconfigs.MachineConfig, port int) error {
	// managed by gvproxy on this backend, so nothing to do
	return nil
}

func resizeDisk(newSize strongunits.GiB, imagePath *define.VMFile) error {
	resize := exec.Command("powershell", []string{"-command", fmt.Sprintf("Resize-VHD \"%s\" %d", imagePath.GetPath(), newSize.ToBytes())}...)
	logrus.Debug(resize.Args)
	resize.Stdout = os.Stdout
	resize.Stderr = os.Stderr
	if err := resize.Run(); err != nil {
		return fmt.Errorf("resizing image: %q", err)
	}
	return nil
}

// removeNetworkAndReadySocketsFromRegistry removes the Network and Ready sockets
// from the Windows Registry
func removeNetworkAndReadySocketsFromRegistry(mc *vmconfigs.MachineConfig) {
	// Remove the HVSOCK for networking
	if err := mc.HyperVHypervisor.NetworkVSock.Remove(); err != nil {
		logrus.Errorf("unable to remove registry entry for %s: %q", mc.HyperVHypervisor.NetworkVSock.KeyName, err)
	}

	// Remove the HVSOCK for events
	if err := mc.HyperVHypervisor.ReadyVsock.Remove(); err != nil {
		logrus.Errorf("unable to remove registry entry for %s: %q", mc.HyperVHypervisor.ReadyVsock.KeyName, err)
	}
}

// readAndSplitIgnition reads the ignition file and splits it into key:value pairs
func readAndSplitIgnition(mc *vmconfigs.MachineConfig, vm *hypervctl.VirtualMachine) error {
	ignFile, err := mc.IgnitionFile()
	if err != nil {
		return err
	}
	ign, err := ignFile.Read()
	if err != nil {
		return err
	}
	reader := bytes.NewReader(ign)

	return vm.SplitAndAddIgnition("ignition.config.", reader)
}

func removeIgnitionFromRegistry(vm *hypervctl.VirtualMachine) error {
	// because the vm is down at this point, we cannot query hyperv for these key value pairs.
	// therefore we blindly iterate from 0-50 and delete the key/value pairs. hyperv does not
	// raise an error if the key is not present
	//
	for i := 0; i < 50; i++ {
		// this is a well known "key" defined in libhvee and is the vm name
		// plus an index starting at 0
		key := fmt.Sprintf("%s%d", vm.ElementName, i)
		if err := vm.RemoveKeyValuePairNoWait(key); err != nil {
			return err
		}
	}
	return nil
}

func logCommandToFile(c *exec.Cmd, filename string) error {
	dir, err := env.GetDataDir(define.HyperVVirt)
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

func createNetworkUnit(netPort uint64) (string, error) {
	netUnit := parser.NewUnitFile()
	netUnit.Add("Unit", "Description", "vsock_network")
	netUnit.Add("Unit", "After", "NetworkManager.service")
	netUnit.Add("Service", "ExecStart", fmt.Sprintf("/usr/libexec/podman/gvforwarder -preexisting -iface vsock0 -url vsock://2:%d/connect", netPort))
	netUnit.Add("Service", "ExecStartPost", "/usr/bin/nmcli c up vsock0")
	netUnit.Add("Install", "WantedBy", "multi-user.target")
	return netUnit.ToString()
}

func (h HyperVStubber) GetRosetta(mc *vmconfigs.MachineConfig) (bool, error) {
	return false, nil
}
