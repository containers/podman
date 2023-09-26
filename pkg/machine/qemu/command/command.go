package command

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/containers/common/libnetwork/etchosts"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/pkg/machine/define"
)

const (
	FdVlanNetdev = "socket,id=vlan,fd=3"
	vlanMac      = "5a:94:ef:e4:0c:ee"
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
func (q *QemuCmd) SetNetwork(vlanSocket *define.VMFile) error {
	// Right now the mac address is hardcoded so that the host networking gives it a specific IP address.  This is
	// why we can only run one vm at a time right now
	if UseFdVLan() {
		*q = append(*q, []string{"-netdev", FdVlanNetdev}...)
	} else {
		if vlanSocket == nil {
			return errors.New("vlanSocket is undefined")
		}
		*q = append(*q, []string{"-netdev", socketVlanNetdev(vlanSocket.GetPath())}...)
	}
	*q = append(*q, []string{"-device", "virtio-net-pci,netdev=vlan,mac=" + vlanMac}...)
	return nil
}

func socketVlanNetdev(path string) string {
	return fmt.Sprintf("stream,id=vlan,server=off,addr.type=unix,addr.path=%s", path)
}

func (q *QemuCmd) SetUSBHostPassthrough(usbs []USBConfig) {
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

type USBConfig struct {
	Bus       string
	DevNumber string
	Vendor    int
	Product   int
}

func ParseUSBs(usbs []string) ([]USBConfig, error) {
	configs := []USBConfig{}
	for _, str := range usbs {
		if str == "" {
			// Ignore --usb="" as it can be used to reset USBConfigs
			continue
		}

		vals := strings.Split(str, ",")
		if len(vals) != 2 {
			return configs, fmt.Errorf("usb: fail to parse: missing ',': %s", str)
		}

		left := strings.Split(vals[0], "=")
		if len(left) != 2 {
			return configs, fmt.Errorf("usb: fail to parse: missing '=': %s", str)
		}

		right := strings.Split(vals[1], "=")
		if len(right) != 2 {
			return configs, fmt.Errorf("usb: fail to parse: missing '=': %s", str)
		}

		option := left[0] + "_" + right[0]

		switch option {
		case "bus_devnum", "devnum_bus":
			bus, devnumber := left[1], right[1]
			if right[0] == "bus" {
				bus, devnumber = devnumber, bus
			}

			configs = append(configs, USBConfig{
				Bus:       bus,
				DevNumber: devnumber,
			})
		case "vendor_product", "product_vendor":
			vendorStr, productStr := left[1], right[1]
			if right[0] == "vendor" {
				vendorStr, productStr = productStr, vendorStr
			}

			vendor, err := strconv.ParseInt(vendorStr, 16, 0)
			if err != nil {
				return configs, fmt.Errorf("usb: fail to convert vendor of %s: %s", str, err)
			}

			product, err := strconv.ParseInt(productStr, 16, 0)
			if err != nil {
				return configs, fmt.Errorf("usb: fail to convert product of %s: %s", str, err)
			}

			configs = append(configs, USBConfig{
				Vendor:  int(vendor),
				Product: int(product),
			})
		default:
			return configs, fmt.Errorf("usb: fail to parse: %s", str)
		}
	}
	return configs, nil
}

func GetProxyVariables() map[string]string {
	proxyOpts := make(map[string]string)
	for _, variable := range config.ProxyEnv {
		if value, ok := os.LookupEnv(variable); ok {
			if value == "" {
				continue
			}

			v := strings.ReplaceAll(value, "127.0.0.1", etchosts.HostContainersInternal)
			v = strings.ReplaceAll(v, "localhost", etchosts.HostContainersInternal)
			proxyOpts[variable] = v
		}
	}
	return proxyOpts
}

// propagateHostEnv is here for providing the ability to propagate
// proxy and SSL settings (e.g. HTTP_PROXY and others) on a start
// and avoid a need of re-creating/re-initiating a VM
func propagateHostEnv(cmdLine QemuCmd) QemuCmd {
	varsToPropagate := make([]string, 0)

	for k, v := range GetProxyVariables() {
		varsToPropagate = append(varsToPropagate, fmt.Sprintf("%s=%q", k, v))
	}

	if sslCertFile, ok := os.LookupEnv("SSL_CERT_FILE"); ok {
		pathInVM := filepath.Join(define.UserCertsTargetPath, filepath.Base(sslCertFile))
		varsToPropagate = append(varsToPropagate, fmt.Sprintf("%s=%q", "SSL_CERT_FILE", pathInVM))
	}

	if _, ok := os.LookupEnv("SSL_CERT_DIR"); ok {
		varsToPropagate = append(varsToPropagate, fmt.Sprintf("%s=%q", "SSL_CERT_DIR", define.UserCertsTargetPath))
	}

	if len(varsToPropagate) > 0 {
		prefix := "name=opt/com.coreos/environment,string="
		envVarsJoined := strings.Join(varsToPropagate, "|")
		fwCfgArg := prefix + base64.StdEncoding.EncodeToString([]byte(envVarsJoined))
		return append(cmdLine, "-fw_cfg", fwCfgArg)
	}

	return cmdLine
}

type Monitor struct {
	//	Address portion of the qmp monitor (/tmp/tmp.sock)
	Address define.VMFile
	// Network portion of the qmp monitor (unix)
	Network string
	// Timeout in seconds for qmp monitor transactions
	Timeout time.Duration
}
