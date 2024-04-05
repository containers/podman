//go:build darwin

package apple

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/strongunits"
	gvproxy "github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/ignition"
	"github.com/containers/podman/v5/pkg/machine/sockets"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"github.com/containers/podman/v5/pkg/systemd/parser"
	vfConfig "github.com/crc-org/vfkit/pkg/config"
	"github.com/sirupsen/logrus"
)

const applehvMACAddress = "5a:94:ef:e4:0c:ee"

var (
	gvProxyWaitBackoff        = 500 * time.Millisecond
	gvProxyMaxBackoffAttempts = 6
	ignitionSocketName        = "ignition.sock"
)

// ResizeDisk uses os truncate to resize (only larger) a raw disk.  the input size
// is assumed GiB
func ResizeDisk(mc *vmconfigs.MachineConfig, newSize strongunits.GiB) error {
	logrus.Debugf("resizing %s to %d bytes", mc.ImagePath.GetPath(), newSize.ToBytes())
	return os.Truncate(mc.ImagePath.GetPath(), int64(newSize.ToBytes()))
}

func SetProviderAttrs(mc *vmconfigs.MachineConfig, opts define.SetOptions, state define.Status) error {
	if state != define.Stopped {
		return errors.New("unable to change settings unless vm is stopped")
	}

	if opts.DiskSize != nil {
		if err := ResizeDisk(mc, *opts.DiskSize); err != nil {
			return err
		}
	}

	if opts.Rootful != nil && mc.HostUser.Rootful != *opts.Rootful {
		if err := mc.SetRootful(*opts.Rootful); err != nil {
			return err
		}
	}

	if opts.USBs != nil {
		return fmt.Errorf("changing USBs not supported for applehv machines")
	}

	// VFKit does not require saving memory, disk, or cpu
	return nil
}

func GenerateSystemDFilesForVirtiofsMounts(mounts []machine.VirtIoFs) ([]ignition.Unit, error) {
	// mounting in fcos with virtiofs is a bit of a dance.  we need a unit file for the mount, a unit file
	// for automatic mounting on boot, and a "preparatory" service file that disables FCOS security, performs
	// the mkdir of the mount point, and then re-enables security.  This must be done for each mount.

	unitFiles := make([]ignition.Unit, 0, len(mounts))
	for _, mnt := range mounts {
		// Here we are looping the mounts and for each mount, we are adding two unit files
		// for virtiofs.  One unit file is the mount itself and the second is to automount it
		// on boot.
		autoMountUnit := parser.NewUnitFile()
		autoMountUnit.Add("Automount", "Where", "%s")
		autoMountUnit.Add("Install", "WantedBy", "multi-user.target")
		autoMountUnit.Add("Unit", "Description", "Mount virtiofs volume %s")
		autoMountUnitFile, err := autoMountUnit.ToString()
		if err != nil {
			return nil, err
		}

		mountUnit := parser.NewUnitFile()
		mountUnit.Add("Mount", "What", "%s")
		mountUnit.Add("Mount", "Where", "%s")
		mountUnit.Add("Mount", "Type", "virtiofs")
		mountUnit.Add("Mount", "Options", "context=\"system_u:object_r:nfs_t:s0\"")
		mountUnit.Add("Install", "WantedBy", "multi-user.target")
		mountUnitFile, err := mountUnit.ToString()
		if err != nil {
			return nil, err
		}

		virtiofsAutomount := ignition.Unit{
			Enabled:  ignition.BoolToPtr(true),
			Name:     fmt.Sprintf("%s.automount", parser.PathEscape(mnt.Target)),
			Contents: ignition.StrToPtr(fmt.Sprintf(autoMountUnitFile, mnt.Tag, mnt.Target)),
		}
		virtiofsMount := ignition.Unit{
			Enabled:  ignition.BoolToPtr(true),
			Name:     fmt.Sprintf("%s.mount", parser.PathEscape(mnt.Target)),
			Contents: ignition.StrToPtr(fmt.Sprintf(mountUnitFile, mnt.Tag, mnt.Target)),
		}

		// This "unit" simulates something like systemctl enable virtiofs-mount-prepare@
		enablePrep := ignition.Unit{
			Enabled: ignition.BoolToPtr(true),
			Name:    fmt.Sprintf("virtiofs-mount-prepare@%s.service", parser.PathEscape(mnt.Target)),
		}

		unitFiles = append(unitFiles, virtiofsAutomount, virtiofsMount, enablePrep)
	}

	// mount prep is a way to workaround the FCOS limitation of creating directories
	// at the rootfs / and then mounting to them.
	mountPrep := parser.NewUnitFile()
	mountPrep.Add("Unit", "Description", "Allow virtios to mount to /")
	mountPrep.Add("Unit", "DefaultDependencies", "no")
	mountPrep.Add("Unit", "ConditionPathExists", "!%f")

	mountPrep.Add("Service", "Type", "oneshot")
	mountPrep.Add("Service", "ExecStartPre", "chattr -i /")
	mountPrep.Add("Service", "ExecStart", "mkdir -p '%f'")
	mountPrep.Add("Service", "ExecStopPost", "chattr +i /")

	mountPrep.Add("Install", "WantedBy", "remote-fs.target")
	mountPrepFile, err := mountPrep.ToString()
	if err != nil {
		return nil, err
	}

	virtioFSChattr := ignition.Unit{
		Contents: ignition.StrToPtr(mountPrepFile),
		Name:     "virtiofs-mount-prepare@.service",
	}
	unitFiles = append(unitFiles, virtioFSChattr)

	return unitFiles, nil
}

// StartGenericAppleVM is wrappered by apple provider methods and starts the vm
func StartGenericAppleVM(mc *vmconfigs.MachineConfig, cmdBinary string, bootloader vfConfig.Bootloader, endpoint string) (func() error, func() error, error) {
	var (
		ignitionSocket *define.VMFile
	)

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
	if err := sockets.WaitForSocketWithBackoffs(gvProxyMaxBackoffAttempts, gvProxyWaitBackoff, gvproxySocket.GetPath(), "gvproxy"); err != nil {
		return nil, nil, err
	}

	netDevice.SetUnixSocketPath(gvproxySocket.GetPath())

	// create a one-time virtual machine for starting because we dont want all this information in the
	// machineconfig if possible.  the preference was to derive this stuff
	vm := vfConfig.NewVirtualMachine(uint(mc.Resources.CPUs), uint64(mc.Resources.Memory), bootloader)

	defaultDevices, readySocket, err := GetDefaultDevices(mc)
	if err != nil {
		return nil, nil, err
	}

	vm.Devices = append(vm.Devices, defaultDevices...)
	vm.Devices = append(vm.Devices, netDevice)

	mounts, err := VirtIOFsToVFKitVirtIODevice(mc.Mounts)
	if err != nil {
		return nil, nil, err
	}
	vm.Devices = append(vm.Devices, mounts...)

	// To start the VM, we need to call vfkit
	cfg, err := config.Default()
	if err != nil {
		return nil, nil, err
	}

	cmdBinaryPath, err := cfg.FindHelperBinary(cmdBinary, true)
	if err != nil {
		return nil, nil, err
	}

	logrus.Debugf("helper binary path is: %s", cmdBinaryPath)

	cmd, err := vm.Cmd(cmdBinaryPath)
	if err != nil {
		return nil, nil, err
	}
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	endpointArgs, err := GetVfKitEndpointCMDArgs(endpoint)
	if err != nil {
		return nil, nil, err
	}

	machineDataDir, err := mc.DataDir()
	if err != nil {
		return nil, nil, err
	}

	cmd.Args = append(cmd.Args, endpointArgs...)

	firstBoot, err := mc.IsFirstBoot()
	if err != nil {
		return nil, nil, err
	}

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		debugDevArgs, err := GetDebugDevicesCMDArgs()
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

		ignitionVsockDeviceCLI, err := GetIgnitionVsockDeviceAsCLI(ignitionSocket.GetPath())
		if err != nil {
			return nil, nil, err
		}
		cmd.Args = append(cmd.Args, ignitionVsockDeviceCLI...)

		logrus.Debug("first boot detected")
		logrus.Debugf("serving ignition file over %s", ignitionSocket.GetPath())
		go func() {
			if err := ServeIgnitionOverSock(ignitionSocket, mc); err != nil {
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

	logrus.Debugf("helper command-line: %v", cmd.Args)

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
				if err := CheckProcessRunning(cmdBinary, cmd.Process.Pid); err != nil {
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

// CheckProcessRunning checks non blocking if the pid exited
// returns nil if process is running otherwise an error if not
func CheckProcessRunning(processName string, pid int) error {
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

// StartGenericNetworking is wrappered by apple provider methods
func StartGenericNetworking(mc *vmconfigs.MachineConfig, cmd *gvproxy.GvproxyCommand) error {
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
