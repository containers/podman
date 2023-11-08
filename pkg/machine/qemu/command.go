package qemu

import (
	"fmt"
	"strconv"

	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/define"
)

// QemuCmd is an alias around a string slice to prevent the need to migrate the
// MachineVM struct due to changes
type QemuCmd []string

// NewQemuBuilder creates a new QemuCmd object that we will build on top of,
// starting with the qemu binary, architecture specific options, and propagated
// proxy and SSL settings
func NewQemuBuilder(binary string, options []string) QemuCmd {
	q := QemuCmd{binary}
	return append(q, options...)
}

// SetMemory adds the specified amount of memory for the machine
func (q *QemuCmd) SetMemory(m uint64) {
	*q = append(*q, "-m", strconv.FormatUint(m, 10))
}

// SetCPUs adds the number of CPUs the machine will have
func (q *QemuCmd) SetCPUs(c uint64) {
	*q = append(*q, "-smp", strconv.FormatUint(c, 10))
}

// SetIgnitionFile specifies the machine's ignition file
func (q *QemuCmd) SetIgnitionFile(file define.VMFile) {
	*q = append(*q, "-fw_cfg", "name=opt/com.coreos/config,file="+file.GetPath())
}

// SetQmpMonitor specifies the machine's qmp socket
func (q *QemuCmd) SetQmpMonitor(monitor Monitor) {
	*q = append(*q, "-qmp", monitor.Network+":"+monitor.Address.GetPath()+",server=on,wait=off")
}

// SetNetwork adds a network device to the machine
func (q *QemuCmd) SetNetwork() {
	// Right now the mac address is hardcoded so that the host networking gives it a specific IP address.  This is
	// why we can only run one vm at a time right now
	*q = append(*q, "-netdev", "socket,id=vlan,fd=3", "-device", "virtio-net-pci,netdev=vlan,mac=5a:94:ef:e4:0c:ee")
}

// SetNetwork adds a network device to the machine
func (q *QemuCmd) SetUSBHostPassthrough(usbs []machine.USBConfig) {
	if len(usbs) == 0 {
		return
	}
	// Add xhci usb emulation first and then each usb device
	*q = append(*q, "-device", "qemu-xhci")
	for _, usb := range usbs {
		var dev string
		if usb.Bus != "" && usb.DevNumber != "" {
			dev = fmt.Sprintf("usb-host,hostbus=%s,hostaddr=%s", usb.Bus, usb.DevNumber)
		} else {
			dev = fmt.Sprintf("usb-host,vendorid=%d,productid=%d", usb.Vendor, usb.Product)
		}
		*q = append(*q, "-device", dev)
	}
}

// SetSerialPort adds a serial port to the machine for readiness
func (q *QemuCmd) SetSerialPort(readySocket, vmPidFile define.VMFile, name string) {
	*q = append(*q,
		"-device", "virtio-serial",
		// qemu needs to establish the long name; other connections can use the symlink'd
		// Note both id and chardev start with an extra "a" because qemu requires that it
		// starts with a letter but users can also use numbers
		"-chardev", "socket,path="+readySocket.GetPath()+",server=on,wait=off,id=a"+name+"_ready",
		"-device", "virtserialport,chardev=a"+name+"_ready"+",name=org.fedoraproject.port.0",
		"-pidfile", vmPidFile.GetPath())
}

// SetVirtfsMount adds a virtfs mount to the machine
func (q *QemuCmd) SetVirtfsMount(source, tag, securityModel string, readonly bool) {
	virtfsOptions := fmt.Sprintf("local,path=%s,mount_tag=%s,security_model=%s", source, tag, securityModel)
	if readonly {
		virtfsOptions += ",readonly"
	}
	*q = append(*q, "-virtfs", virtfsOptions)
}

// SetBootableImage specifies the image the machine will use to boot
func (q *QemuCmd) SetBootableImage(image string) {
	*q = append(*q, "-drive", "if=virtio,file="+image)
}

// SetDisplay specifies whether the machine will have a display
func (q *QemuCmd) SetDisplay(display string) {
	*q = append(*q, "-display", display)
}

// SetPropagatedHostEnvs adds options that propagate SSL and proxy settings
func (q *QemuCmd) SetPropagatedHostEnvs() {
	*q = propagateHostEnv(*q)
}

func (q *QemuCmd) Build() []string {
	return *q
}
