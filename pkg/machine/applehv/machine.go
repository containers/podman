//go:build darwin

package applehv

import (
	"fmt"
	"os"
	"syscall"

	"github.com/containers/common/pkg/strongunits"
	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/ignition"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"github.com/containers/podman/v5/pkg/systemd/parser"
	vfRest "github.com/crc-org/vfkit/pkg/rest"
	"github.com/sirupsen/logrus"
)

func (a *AppleHVStubber) Remove(mc *vmconfigs.MachineConfig) ([]string, func() error, error) {
	mc.Lock()
	defer mc.Unlock()

	// TODO we could delete the vfkit pid/log files if we wanted to be thorough
	return []string{}, func() error { return nil }, nil
}

// getIgnitionVsockDeviceAsCLI retrieves the ignition vsock device and converts
// it to a cmdline format
func getIgnitionVsockDeviceAsCLI(ignitionSocketPath string) ([]string, error) {
	ignitionVsockDevice, err := getIgnitionVsockDevice(ignitionSocketPath)
	if err != nil {
		return nil, err
	}
	// Convert the device into cli args
	ignitionVsockDeviceCLI, err := ignitionVsockDevice.ToCmdLine()
	if err != nil {
		return nil, err
	}
	return ignitionVsockDeviceCLI, nil
}

// getDebugDevicesCMDArgs retrieves the debug devices and converts them to a
// cmdline format
func getDebugDevicesCMDArgs() ([]string, error) {
	args := []string{}
	debugDevices, err := getDebugDevices()
	if err != nil {
		return nil, err
	}
	for _, debugDevice := range debugDevices {
		debugCli, err := debugDevice.ToCmdLine()
		if err != nil {
			return nil, err
		}
		args = append(args, debugCli...)
	}
	return args, nil
}

// getVfKitEndpointCMDArgs converts the vfkit endpoint to a cmdline format
func getVfKitEndpointCMDArgs(endpoint string) ([]string, error) {
	restEndpoint, err := vfRest.NewEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	return restEndpoint.ToCmdLine()
}

func (a *AppleHVStubber) State(mc *vmconfigs.MachineConfig, _ bool) (define.Status, error) {
	vmStatus, err := mc.AppleHypervisor.Vfkit.State()
	if err != nil {
		return "", err
	}
	return vmStatus, nil
}

func (a *AppleHVStubber) StopVM(mc *vmconfigs.MachineConfig, _ bool) error {
	mc.Lock()
	defer mc.Unlock()
	return mc.AppleHypervisor.Vfkit.Stop(false, true)
}

// checkProcessRunning checks non blocking if the pid exited
// returns nil if process is running otherwise an error if not
func checkProcessRunning(processName string, pid int) error {
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

// resizeDisk uses os truncate to resize (only larger) a raw disk.  the input size
// is assumed GiB
func resizeDisk(mc *vmconfigs.MachineConfig, newSize strongunits.GiB) error {
	logrus.Debugf("resizing %s to %d bytes", mc.ImagePath.GetPath(), newSize.ToBytes())
	return os.Truncate(mc.ImagePath.GetPath(), int64(newSize.ToBytes()))
}

func generateSystemDFilesForVirtiofsMounts(mounts []machine.VirtIoFs) []ignition.Unit {
	// mounting in fcos with virtiofs is a bit of a dance.  we need a unit file for the mount, a unit file
	// for automatic mounting on boot, and a "preparatory" service file that disables FCOS security, performs
	// the mkdir of the mount point, and then re-enables security.  This must be done for each mount.

	var unitFiles []ignition.Unit
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
			logrus.Warnf(err.Error())
		}

		mountUnit := parser.NewUnitFile()
		mountUnit.Add("Mount", "What", "%s")
		mountUnit.Add("Mount", "Where", "%s")
		mountUnit.Add("Mount", "Type", "virtiofs")
		mountUnit.Add("Mount", "Options", "defcontext=\"system_u:object_r:nfs_t:s0\"")
		mountUnit.Add("Install", "WantedBy", "multi-user.target")
		mountUnitFile, err := mountUnit.ToString()
		if err != nil {
			logrus.Warnf(err.Error())
		}

		virtiofsAutomount := ignition.Unit{
			Enabled:  ignition.BoolToPtr(true),
			Name:     fmt.Sprintf("%s.automount", mnt.Tag),
			Contents: ignition.StrToPtr(fmt.Sprintf(autoMountUnitFile, mnt.Target, mnt.Target)),
		}
		virtiofsMount := ignition.Unit{
			Enabled:  ignition.BoolToPtr(true),
			Name:     fmt.Sprintf("%s.mount", mnt.Tag),
			Contents: ignition.StrToPtr(fmt.Sprintf(mountUnitFile, mnt.Tag, mnt.Target)),
		}

		// This "unit" simulates something like systemctl enable virtiofs-mount-prepare@
		enablePrep := ignition.Unit{
			Enabled: ignition.BoolToPtr(true),
			Name:    fmt.Sprintf("virtiofs-mount-prepare@%s.service", mnt.Tag),
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
		logrus.Warnf(err.Error())
	}

	virtioFSChattr := ignition.Unit{
		Contents: ignition.StrToPtr(mountPrepFile),
		Name:     "virtiofs-mount-prepare@.service",
	}
	unitFiles = append(unitFiles, virtioFSChattr)

	return unitFiles
}
