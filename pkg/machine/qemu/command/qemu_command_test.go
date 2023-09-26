//go:build (amd64 && !windows) || (arm64 && !windows)

package command

import (
	"fmt"
	"testing"

	"github.com/containers/podman/v4/pkg/machine/define"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQemuCmd(t *testing.T) {
	ignFile, err := define.NewMachineFile(t.TempDir()+"demo-ignition-file.ign", nil)
	assert.NoError(t, err)

	machineAddrFile, err := define.NewMachineFile(t.TempDir()+"tmp.sock", nil)
	assert.NoError(t, err)

	readySocket, err := define.NewMachineFile(t.TempDir()+"readySocket.sock", nil)
	assert.NoError(t, err)

	vmPidFile, err := define.NewMachineFile(t.TempDir()+"vmpidfile.pid", nil)
	assert.NoError(t, err)

	monitor := Monitor{
		Address: *machineAddrFile,
		Network: "unix",
		Timeout: 3,
	}
	ignPath := ignFile.GetPath()
	addrFilePath := machineAddrFile.GetPath()
	readySocketPath := readySocket.GetPath()
	vmPidFilePath := vmPidFile.GetPath()
	bootableImagePath := t.TempDir() + "test-machine_fedora-coreos-38.20230918.2.0-qemu.x86_64.qcow2"

	cmd := NewQemuBuilder("/usr/bin/qemu-system-x86_64", []string{})
	cmd.SetMemory(2048)
	cmd.SetCPUs(4)
	cmd.SetIgnitionFile(*ignFile)
	cmd.SetQmpMonitor(monitor)
	err = cmd.SetNetwork(nil)
	assert.NoError(t, err)
	cmd.SetSerialPort(*readySocket, *vmPidFile, "test-machine")
	cmd.SetVirtfsMount("/tmp/path", "vol10", "none", true)
	cmd.SetBootableImage(bootableImagePath)
	cmd.SetDisplay("none")

	expected := []string{
		"/usr/bin/qemu-system-x86_64",
		"-m", "2048",
		"-smp", "4",
		"-fw_cfg", fmt.Sprintf("name=opt/com.coreos/config,file=%s", ignPath),
		"-qmp", fmt.Sprintf("unix:%s,server=on,wait=off", addrFilePath),
		"-netdev", "socket,id=vlan,fd=3",
		"-device", "virtio-net-pci,netdev=vlan,mac=5a:94:ef:e4:0c:ee",
		"-device", "virtio-serial",
		"-chardev", fmt.Sprintf("socket,path=%s,server=on,wait=off,id=atest-machine_ready", readySocketPath),
		"-device", "virtserialport,chardev=atest-machine_ready,name=org.fedoraproject.port.0",
		"-pidfile", vmPidFilePath,
		"-virtfs", "local,path=/tmp/path,mount_tag=vol10,security_model=none,readonly",
		"-drive", fmt.Sprintf("if=virtio,file=%s", bootableImagePath),
		"-display", "none"}

	require.Equal(t, cmd.Build(), expected)
}

func TestQemuCmdUnixVlanMissingSocket(t *testing.T) {
	t.Setenv("CONTAINERS_USE_SOCKET_VLAN", "true")
	cmd := NewQemuBuilder("/usr/bin/qemu-system-x86_64", []string{})
	err := cmd.SetNetwork(nil)
	assert.Error(t, err)
}

func TestQemuCmdUnixVlan(t *testing.T) {
	t.Setenv("CONTAINERS_USE_SOCKET_VLAN", "true")
	ignFile, err := define.NewMachineFile(t.TempDir()+"demo-ignition-file.ign", nil)
	assert.NoError(t, err)

	machineAddrFile, err := define.NewMachineFile(t.TempDir()+"tmp.sock", nil)
	assert.NoError(t, err)

	vlanSocket, err := define.NewMachineFile(t.TempDir()+"vlan.sock", nil)
	assert.NoError(t, err)

	readySocket, err := define.NewMachineFile(t.TempDir()+"readySocket.sock", nil)
	assert.NoError(t, err)

	vmPidFile, err := define.NewMachineFile(t.TempDir()+"vmpidfile.pid", nil)
	assert.NoError(t, err)

	monitor := Monitor{
		Address: *machineAddrFile,
		Network: "unix",
		Timeout: 3,
	}
	ignPath := ignFile.GetPath()
	addrFilePath := machineAddrFile.GetPath()
	readySocketPath := readySocket.GetPath()
	vmPidFilePath := vmPidFile.GetPath()
	bootableImagePath := t.TempDir() + "test-machine_fedora-coreos-38.20230918.2.0-qemu.x86_64.qcow2"

	cmd := NewQemuBuilder("/usr/bin/qemu-system-x86_64", []string{})
	cmd.SetMemory(2048)
	cmd.SetCPUs(4)
	cmd.SetIgnitionFile(*ignFile)
	cmd.SetQmpMonitor(monitor)
	err = cmd.SetNetwork(vlanSocket)
	assert.NoError(t, err)
	cmd.SetSerialPort(*readySocket, *vmPidFile, "test-machine")
	cmd.SetVirtfsMount("/tmp/path", "vol10", "none", true)
	cmd.SetBootableImage(bootableImagePath)
	cmd.SetDisplay("none")

	expected := []string{
		"/usr/bin/qemu-system-x86_64",
		"-m", "2048",
		"-smp", "4",
		"-fw_cfg", fmt.Sprintf("name=opt/com.coreos/config,file=%s", ignPath),
		"-qmp", fmt.Sprintf("unix:%s,server=on,wait=off", addrFilePath),
		"-netdev", socketVlanNetdev(vlanSocket.GetPath()),
		"-device", "virtio-net-pci,netdev=vlan,mac=5a:94:ef:e4:0c:ee",
		"-device", "virtio-serial",
		"-chardev", fmt.Sprintf("socket,path=%s,server=on,wait=off,id=atest-machine_ready", readySocketPath),
		"-device", "virtserialport,chardev=atest-machine_ready,name=org.fedoraproject.port.0",
		"-pidfile", vmPidFilePath,
		"-virtfs", "local,path=/tmp/path,mount_tag=vol10,security_model=none,readonly",
		"-drive", fmt.Sprintf("if=virtio,file=%s", bootableImagePath),
		"-display", "none"}

	require.Equal(t, cmd.Build(), expected)
}
