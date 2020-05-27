package generate

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/libpod/pkg/rootless"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/devices"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func u32Ptr(i int64) *uint32     { u := uint32(i); return &u }
func fmPtr(i int64) *os.FileMode { fm := os.FileMode(i); return &fm }

// Device transforms a libcontainer configs.Device to a specs.LinuxDevice object.
func Device(d *configs.Device) spec.LinuxDevice {
	return spec.LinuxDevice{
		Type:     string(d.Type),
		Path:     d.Path,
		Major:    d.Major,
		Minor:    d.Minor,
		FileMode: fmPtr(int64(d.FileMode)),
		UID:      u32Ptr(int64(d.Uid)),
		GID:      u32Ptr(int64(d.Gid)),
	}
}

func addPrivilegedDevices(g *generate.Generator) error {
	hostDevices, err := getDevices("/dev")
	if err != nil {
		return err
	}
	g.ClearLinuxDevices()

	if rootless.IsRootless() {
		mounts := make(map[string]interface{})
		for _, m := range g.Mounts() {
			mounts[m.Destination] = true
		}
		newMounts := []spec.Mount{}
		for _, d := range hostDevices {
			devMnt := spec.Mount{
				Destination: d.Path,
				Type:        TypeBind,
				Source:      d.Path,
				Options:     []string{"slave", "nosuid", "noexec", "rw", "rbind"},
			}
			if d.Path == "/dev/ptmx" || strings.HasPrefix(d.Path, "/dev/tty") {
				continue
			}
			if _, found := mounts[d.Path]; found {
				continue
			}
			st, err := os.Stat(d.Path)
			if err != nil {
				if err == unix.EPERM {
					continue
				}
				return errors.Wrapf(err, "stat %s", d.Path)
			}
			// Skip devices that the user has not access to.
			if st.Mode()&0007 == 0 {
				continue
			}
			newMounts = append(newMounts, devMnt)
		}
		g.Config.Mounts = append(newMounts, g.Config.Mounts...)
		if g.Config.Linux.Resources != nil {
			g.Config.Linux.Resources.Devices = nil
		}
	} else {
		for _, d := range hostDevices {
			g.AddDevice(Device(d))
		}
		// Add resources device - need to clear the existing one first.
		if g.Config.Linux.Resources != nil {
			g.Config.Linux.Resources.Devices = nil
		}
		g.AddLinuxResourcesDevice(true, "", nil, nil, "rwm")
	}

	return nil
}

// DevicesFromPath computes a list of devices
func DevicesFromPath(g *generate.Generator, devicePath string) error {
	devs := strings.Split(devicePath, ":")
	resolvedDevicePath := devs[0]
	// check if it is a symbolic link
	if src, err := os.Lstat(resolvedDevicePath); err == nil && src.Mode()&os.ModeSymlink == os.ModeSymlink {
		if linkedPathOnHost, err := filepath.EvalSymlinks(resolvedDevicePath); err == nil {
			resolvedDevicePath = linkedPathOnHost
		}
	}
	st, err := os.Stat(resolvedDevicePath)
	if err != nil {
		return errors.Wrapf(err, "cannot stat %s", devicePath)
	}
	if st.IsDir() {
		found := false
		src := resolvedDevicePath
		dest := src
		var devmode string
		if len(devs) > 1 {
			if len(devs[1]) > 0 && devs[1][0] == '/' {
				dest = devs[1]
			} else {
				devmode = devs[1]
			}
		}
		if len(devs) > 2 {
			if devmode != "" {
				return errors.Wrapf(unix.EINVAL, "invalid device specification %s", devicePath)
			}
			devmode = devs[2]
		}

		// mount the internal devices recursively
		if err := filepath.Walk(resolvedDevicePath, func(dpath string, f os.FileInfo, e error) error {

			if f.Mode()&os.ModeDevice == os.ModeDevice {
				found = true
				device := fmt.Sprintf("%s:%s", dpath, filepath.Join(dest, strings.TrimPrefix(dpath, src)))
				if devmode != "" {
					device = fmt.Sprintf("%s:%s", device, devmode)
				}
				if err := addDevice(g, device); err != nil {
					return errors.Wrapf(err, "failed to add %s device", dpath)
				}
			}
			return nil
		}); err != nil {
			return err
		}
		if !found {
			return errors.Wrapf(unix.EINVAL, "no devices found in %s", devicePath)
		}
		return nil
	}

	return addDevice(g, strings.Join(append([]string{resolvedDevicePath}, devs[1:]...), ":"))
}

func BlockAccessToKernelFilesystems(privileged, pidModeIsHost bool, g *generate.Generator) {
	if !privileged {
		for _, mp := range []string{
			"/proc/acpi",
			"/proc/kcore",
			"/proc/keys",
			"/proc/latency_stats",
			"/proc/timer_list",
			"/proc/timer_stats",
			"/proc/sched_debug",
			"/proc/scsi",
			"/sys/firmware",
			"/sys/fs/selinux",
		} {
			g.AddLinuxMaskedPaths(mp)
		}

		if pidModeIsHost && rootless.IsRootless() {
			return
		}

		for _, rp := range []string{
			"/proc/asound",
			"/proc/bus",
			"/proc/fs",
			"/proc/irq",
			"/proc/sys",
			"/proc/sysrq-trigger",
		} {
			g.AddLinuxReadonlyPaths(rp)
		}
	}
}

// based on getDevices from runc (libcontainer/devices/devices.go)
func getDevices(path string) ([]*configs.Device, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		if rootless.IsRootless() && os.IsPermission(err) {
			return nil, nil
		}
		return nil, err
	}
	out := []*configs.Device{}
	for _, f := range files {
		switch {
		case f.IsDir():
			switch f.Name() {
			// ".lxc" & ".lxd-mounts" added to address https://github.com/lxc/lxd/issues/2825
			case "pts", "shm", "fd", "mqueue", ".lxc", ".lxd-mounts":
				continue
			default:
				sub, err := getDevices(filepath.Join(path, f.Name()))
				if err != nil {
					return nil, err
				}
				if sub != nil {
					out = append(out, sub...)
				}
				continue
			}
		case f.Name() == "console":
			continue
		}
		device, err := devices.DeviceFromPath(filepath.Join(path, f.Name()), "rwm")
		if err != nil {
			if err == devices.ErrNotADevice {
				continue
			}
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		out = append(out, device)
	}
	return out, nil
}

func addDevice(g *generate.Generator, device string) error {
	src, dst, permissions, err := ParseDevice(device)
	if err != nil {
		return err
	}
	dev, err := devices.DeviceFromPath(src, permissions)
	if err != nil {
		return errors.Wrapf(err, "%s is not a valid device", src)
	}
	if rootless.IsRootless() {
		if _, err := os.Stat(src); err != nil {
			if os.IsNotExist(err) {
				return errors.Wrapf(err, "the specified device %s doesn't exist", src)
			}
			return errors.Wrapf(err, "stat device %s exist", src)
		}
		perm := "ro"
		if strings.Contains(permissions, "w") {
			perm = "rw"
		}
		devMnt := spec.Mount{
			Destination: dst,
			Type:        TypeBind,
			Source:      src,
			Options:     []string{"slave", "nosuid", "noexec", perm, "rbind"},
		}
		g.Config.Mounts = append(g.Config.Mounts, devMnt)
		return nil
	}
	dev.Path = dst
	linuxdev := spec.LinuxDevice{
		Path:     dev.Path,
		Type:     string(dev.Type),
		Major:    dev.Major,
		Minor:    dev.Minor,
		FileMode: &dev.FileMode,
		UID:      &dev.Uid,
		GID:      &dev.Gid,
	}
	g.AddDevice(linuxdev)
	g.AddLinuxResourcesDevice(true, string(dev.Type), &dev.Major, &dev.Minor, dev.Permissions)
	return nil
}

// ParseDevice parses device mapping string to a src, dest & permissions string
func ParseDevice(device string) (string, string, string, error) { //nolint
	src := ""
	dst := ""
	permissions := "rwm"
	arr := strings.Split(device, ":")
	switch len(arr) {
	case 3:
		if !IsValidDeviceMode(arr[2]) {
			return "", "", "", fmt.Errorf("invalid device mode: %s", arr[2])
		}
		permissions = arr[2]
		fallthrough
	case 2:
		if IsValidDeviceMode(arr[1]) {
			permissions = arr[1]
		} else {
			if arr[1][0] != '/' {
				return "", "", "", fmt.Errorf("invalid device mode: %s", arr[1])
			}
			dst = arr[1]
		}
		fallthrough
	case 1:
		src = arr[0]
	default:
		return "", "", "", fmt.Errorf("invalid device specification: %s", device)
	}

	if dst == "" {
		dst = src
	}
	return src, dst, permissions, nil
}

// IsValidDeviceMode checks if the mode for device is valid or not.
// IsValid mode is a composition of r (read), w (write), and m (mknod).
func IsValidDeviceMode(mode string) bool {
	var legalDeviceMode = map[rune]bool{
		'r': true,
		'w': true,
		'm': true,
	}
	if mode == "" {
		return false
	}
	for _, c := range mode {
		if !legalDeviceMode[c] {
			return false
		}
		legalDeviceMode[c] = false
	}
	return true
}
