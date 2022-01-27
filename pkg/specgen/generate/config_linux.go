package generate

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/rootless"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

var (
	errNotADevice = errors.New("not a device node")
)

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
				Type:        define.TypeBind,
				Source:      d.Path,
				Options:     []string{"slave", "nosuid", "noexec", "rw", "rbind"},
			}
			if d.Path == "/dev/ptmx" || strings.HasPrefix(d.Path, "/dev/tty") {
				continue
			}
			if _, found := mounts[d.Path]; found {
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
			g.AddDevice(d)
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
		return err
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

func BlockAccessToKernelFilesystems(privileged, pidModeIsHost bool, mask, unmask []string, g *generate.Generator) {
	defaultMaskPaths := []string{"/proc/acpi",
		"/proc/kcore",
		"/proc/keys",
		"/proc/latency_stats",
		"/proc/timer_list",
		"/proc/timer_stats",
		"/proc/sched_debug",
		"/proc/scsi",
		"/sys/firmware",
		"/sys/fs/selinux",
		"/sys/dev/block",
	}

	if !privileged {
		for _, mp := range defaultMaskPaths {
			// check that the path to mask is not in the list of paths to unmask
			if shouldMask(mp, unmask) {
				g.AddLinuxMaskedPaths(mp)
			}
		}
		for _, rp := range []string{
			"/proc/asound",
			"/proc/bus",
			"/proc/fs",
			"/proc/irq",
			"/proc/sys",
			"/proc/sysrq-trigger",
		} {
			if shouldMask(rp, unmask) {
				g.AddLinuxReadonlyPaths(rp)
			}
		}

		if pidModeIsHost && rootless.IsRootless() {
			return
		}
	}

	// mask the paths provided by the user
	for _, mp := range mask {
		if !path.IsAbs(mp) && mp != "" {
			logrus.Errorf("Path %q is not an absolute path, skipping...", mp)
			continue
		}
		g.AddLinuxMaskedPaths(mp)
	}
}

// based on getDevices from runc (libcontainer/devices/devices.go)
func getDevices(path string) ([]spec.LinuxDevice, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		if rootless.IsRootless() && os.IsPermission(err) {
			return nil, nil
		}
		return nil, err
	}
	out := []spec.LinuxDevice{}
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
		case f.Mode()&os.ModeSymlink != 0:
			continue
		}

		device, err := deviceFromPath(filepath.Join(path, f.Name()))
		if err != nil {
			if err == errNotADevice {
				continue
			}
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		out = append(out, *device)
	}
	return out, nil
}

func addDevice(g *generate.Generator, device string) error {
	src, dst, permissions, err := ParseDevice(device)
	if err != nil {
		return err
	}
	dev, err := deviceFromPath(src)
	if err != nil {
		return errors.Wrapf(err, "%s is not a valid device", src)
	}
	if rootless.IsRootless() {
		if _, err := os.Stat(src); err != nil {
			return err
		}
		perm := "ro"
		if strings.Contains(permissions, "w") {
			perm = "rw"
		}
		devMnt := spec.Mount{
			Destination: dst,
			Type:        define.TypeBind,
			Source:      src,
			Options:     []string{"slave", "nosuid", "noexec", perm, "rbind"},
		}
		g.Config.Mounts = append(g.Config.Mounts, devMnt)
		return nil
	} else if src == "/dev/fuse" {
		// if the user is asking for fuse inside the container
		// make sure the module is loaded.
		f, err := unix.Open(src, unix.O_RDONLY|unix.O_NONBLOCK, 0)
		if err == nil {
			unix.Close(f)
		}
	}
	dev.Path = dst
	g.AddDevice(*dev)
	g.AddLinuxResourcesDevice(true, dev.Type, &dev.Major, &dev.Minor, permissions)
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

// Copied from github.com/opencontainers/runc/libcontainer/devices
// Given the path to a device look up the information about a linux device
func deviceFromPath(path string) (*spec.LinuxDevice, error) {
	var stat unix.Stat_t
	err := unix.Lstat(path, &stat)
	if err != nil {
		return nil, err
	}
	var (
		devType   string
		mode      = stat.Mode
		devNumber = uint64(stat.Rdev)
		m         = os.FileMode(mode)
	)

	switch {
	case mode&unix.S_IFBLK == unix.S_IFBLK:
		devType = "b"
	case mode&unix.S_IFCHR == unix.S_IFCHR:
		devType = "c"
	case mode&unix.S_IFIFO == unix.S_IFIFO:
		devType = "p"
	default:
		return nil, errNotADevice
	}

	return &spec.LinuxDevice{
		Type:     devType,
		Path:     path,
		FileMode: &m,
		UID:      &stat.Uid,
		GID:      &stat.Gid,
		Major:    int64(unix.Major(devNumber)),
		Minor:    int64(unix.Minor(devNumber)),
	}, nil
}

func supportAmbientCapabilities() bool {
	err := unix.Prctl(unix.PR_CAP_AMBIENT, unix.PR_CAP_AMBIENT_IS_SET, 0, 0, 0)
	return err == nil
}

func shouldMask(mask string, unmask []string) bool {
	for _, m := range unmask {
		if strings.ToLower(m) == "all" {
			return false
		}
		for _, m1 := range strings.Split(m, ":") {
			match, err := filepath.Match(m1, mask)
			if err != nil {
				logrus.Errorf(err.Error())
			}
			if match {
				return false
			}
		}
	}
	return true
}
