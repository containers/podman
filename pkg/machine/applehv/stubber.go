//go:build darwin

package applehv

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/strongunits"
	gvproxy "github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/applehv/vfkit"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/ignition"
	"github.com/containers/podman/v5/pkg/machine/sockets"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"github.com/containers/podman/v5/utils"
	vfConfig "github.com/crc-org/vfkit/pkg/config"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// applehcMACAddress is a pre-defined mac address that vfkit recognizes
// and is required for network flow
const applehvMACAddress = "5a:94:ef:e4:0c:ee"

var (
	vfkitCommand              = "vfkit"
	gvProxyWaitBackoff        = 500 * time.Millisecond
	gvProxyMaxBackoffAttempts = 6
)

type AppleHVStubber struct {
	vmconfigs.AppleHVConfig
}

func (a AppleHVStubber) CreateVM(opts define.CreateVMOpts, mc *vmconfigs.MachineConfig, ignBuilder *ignition.IgnitionBuilder) error {
	mc.AppleHypervisor = new(vmconfigs.AppleHVConfig)
	mc.AppleHypervisor.Vfkit = vfkit.VfkitHelper{}
	bl := vfConfig.NewEFIBootloader(fmt.Sprintf("%s/efi-bl-%s", opts.Dirs.DataDir.GetPath(), opts.Name), true)
	mc.AppleHypervisor.Vfkit.VirtualMachine = vfConfig.NewVirtualMachine(uint(mc.Resources.CPUs), mc.Resources.Memory, bl)

	randPort, err := utils.GetRandomPort()
	if err != nil {
		return err
	}
	mc.AppleHypervisor.Vfkit.Endpoint = localhostURI + ":" + strconv.Itoa(randPort)

	var virtiofsMounts []machine.VirtIoFs
	for _, mnt := range mc.Mounts {
		virtiofsMounts = append(virtiofsMounts, machine.MountToVirtIOFs(mnt))
	}

	// Populate the ignition file with virtiofs stuff
	ignBuilder.WithUnit(generateSystemDFilesForVirtiofsMounts(virtiofsMounts)...)

	return resizeDisk(mc, strongunits.GiB(mc.Resources.DiskSize))
}

func (a AppleHVStubber) GetHyperVisorVMs() ([]string, error) {
	// not applicable for applehv
	return nil, nil
}

func (a AppleHVStubber) MountType() vmconfigs.VolumeMountType {
	return vmconfigs.VirtIOFS
}

func (a AppleHVStubber) MountVolumesToVM(_ *vmconfigs.MachineConfig, _ bool) error {
	// virtiofs: nothing to do here
	return nil
}

func (a AppleHVStubber) RemoveAndCleanMachines(_ *define.MachineDirs) error {
	return nil
}

func (a AppleHVStubber) SetProviderAttrs(mc *vmconfigs.MachineConfig, cpus, memory *uint64, newDiskSize *strongunits.GiB, newRootful *bool) error {
	if newDiskSize != nil {
		if err := resizeDisk(mc, *newDiskSize); err != nil {
			return err
		}
	}

	if newRootful != nil && mc.HostUser.Rootful != *newRootful {
		if err := mc.SetRootful(*newRootful); err != nil {
			return err
		}
	}

	// VFKit does not require saving memory, disk, or cpu
	return nil
}

func (a AppleHVStubber) StartNetworking(mc *vmconfigs.MachineConfig, cmd *gvproxy.GvproxyCommand) error {
	gvProxySock, err := mc.GVProxySocket()
	if err != nil {
		return err
	}
	// make sure it does not exist before gvproxy is called
	if err := gvProxySock.Delete(); err != nil {
		logrus.Error(err)
	}
	cmd.AddVfkitSocket(fmt.Sprintf("unixgram://%s", gvProxySock.GetPath()))
	return nil
}

func (a AppleHVStubber) StartVM(mc *vmconfigs.MachineConfig) (func() error, func() error, error) {
	var (
		ignitionSocket *define.VMFile
	)

	if bl := mc.AppleHypervisor.Vfkit.VirtualMachine.Bootloader; bl == nil {
		return nil, nil, fmt.Errorf("unable to determine boot loader for this machine")
	}

	// Add networking
	netDevice, err := vfConfig.VirtioNetNew(applehvMACAddress)
	if err != nil {
		return nil, nil, err
	}
	// Set user networking with gvproxy

	gvproxySocket, err := mc.GVProxySocket()
	if err != nil {
		return nil, nil, err
	}

	// Wait on gvproxy to be running and aware
	if err := waitForGvProxy(gvproxySocket); err != nil {
		return nil, nil, err
	}

	netDevice.SetUnixSocketPath(gvproxySocket.GetPath())

	readySocket, err := mc.ReadySocket()
	if err != nil {
		return nil, nil, err
	}

	logfile, err := mc.LogFile()
	if err != nil {
		return nil, nil, err
	}

	// create a one-time virtual machine for starting because we dont want all this information in the
	// machineconfig if possible.  the preference was to derive this stuff
	vm := vfConfig.NewVirtualMachine(uint(mc.Resources.CPUs), mc.Resources.Memory, mc.AppleHypervisor.Vfkit.VirtualMachine.Bootloader)

	defaultDevices, err := getDefaultDevices(mc.ImagePath.GetPath(), logfile.GetPath(), readySocket.GetPath())
	if err != nil {
		return nil, nil, err
	}

	vm.Devices = append(vm.Devices, defaultDevices...)
	vm.Devices = append(vm.Devices, netDevice)

	mounts, err := virtIOFsToVFKitVirtIODevice(mc.Mounts)
	if err != nil {
		return nil, nil, err
	}
	vm.Devices = append(vm.Devices, mounts...)

	// To start the VM, we need to call vfkit
	cfg, err := config.Default()
	if err != nil {
		return nil, nil, err
	}

	vfkitBinaryPath, err := cfg.FindHelperBinary(vfkitCommand, true)
	if err != nil {
		return nil, nil, err
	}

	logrus.Debugf("vfkit path is: %s", vfkitBinaryPath)

	cmd, err := vm.Cmd(vfkitBinaryPath)
	if err != nil {
		return nil, nil, err
	}

	vfkitEndpointArgs, err := getVfKitEndpointCMDArgs(mc.AppleHypervisor.Vfkit.Endpoint)
	if err != nil {
		return nil, nil, err
	}

	machineDataDir, err := mc.DataDir()
	if err != nil {
		return nil, nil, err
	}

	cmd.Args = append(cmd.Args, vfkitEndpointArgs...)

	firstBoot, err := mc.IsFirstBoot()
	if err != nil {
		return nil, nil, err
	}

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		debugDevArgs, err := getDebugDevicesCMDArgs()
		if err != nil {
			return nil, nil, err
		}
		cmd.Args = append(cmd.Args, debugDevArgs...)
		cmd.Args = append(cmd.Args, "--gui") // add command line switch to pop the gui open
	}

	if firstBoot {
		// If this is the first boot of the vm, we need to add the vsock
		// device to vfkit so we can inject the ignition file
		socketName := fmt.Sprintf("%s-%s", mc.Name, ignitionSocketName)
		ignitionSocket, err = machineDataDir.AppendToNewVMFile(socketName, &socketName)
		if err != nil {
			return nil, nil, err
		}
		if err := ignitionSocket.Delete(); err != nil {
			logrus.Errorf("unable to delete ignition socket: %q", err)
		}

		ignitionVsockDeviceCLI, err := getIgnitionVsockDeviceAsCLI(ignitionSocket.GetPath())
		if err != nil {
			return nil, nil, err
		}
		cmd.Args = append(cmd.Args, ignitionVsockDeviceCLI...)

		logrus.Debug("first boot detected")
		logrus.Debugf("serving ignition file over %s", ignitionSocket.GetPath())
		go func() {
			if err := serveIgnitionOverSock(ignitionSocket, mc); err != nil {
				logrus.Error(err)
			}
			logrus.Debug("ignition vsock server exited")
		}()
	}

	logrus.Debugf("listening for ready on: %s", readySocket.GetPath())
	if err := readySocket.Delete(); err != nil {
		logrus.Warnf("unable to delete previous ready socket: %q", err)
	}
	readyListen, err := net.Listen("unix", readySocket.GetPath())
	if err != nil {
		return nil, nil, err
	}

	logrus.Debug("waiting for ready notification")
	readyChan := make(chan error)
	go sockets.ListenAndWaitOnSocket(readyChan, readyListen)

	logrus.Debugf("vfkit command-line: %v", cmd.Args)

	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}

	returnFunc := func() error {
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
			logrus.Debug("ready notification received")
		}
		return nil
	}
	return cmd.Process.Release, returnFunc, nil
}

func (a AppleHVStubber) StopHostNetworking(_ *vmconfigs.MachineConfig, _ define.VMType) error {
	return nil
}

func (a AppleHVStubber) VMType() define.VMType {
	return define.AppleHvVirt
}

func waitForGvProxy(gvproxySocket *define.VMFile) error {
	backoffWait := gvProxyWaitBackoff
	logrus.Debug("checking that gvproxy is running")
	for i := 0; i < gvProxyMaxBackoffAttempts; i++ {
		err := unix.Access(gvproxySocket.GetPath(), unix.W_OK)
		if err == nil {
			return nil
		}
		time.Sleep(backoffWait)
		backoffWait *= 2
	}
	return fmt.Errorf("unable to connect to gvproxy %q", gvproxySocket.GetPath())
}

func (a AppleHVStubber) PrepareIgnition(_ *vmconfigs.MachineConfig, _ *ignition.IgnitionBuilder) (*ignition.ReadyUnitOpts, error) {
	return nil, nil
}

func (a AppleHVStubber) PostStartNetworking(mc *vmconfigs.MachineConfig) error {
	return nil
}
