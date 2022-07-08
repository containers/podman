package generate

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/util"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

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
				return fmt.Errorf("invalid device specification %s: %w", devicePath, unix.EINVAL)
			}
			devmode = devs[2]
		}

		// mount the internal devices recursively
		if err := filepath.WalkDir(resolvedDevicePath, func(dpath string, d fs.DirEntry, e error) error {
			if d.Type()&os.ModeDevice == os.ModeDevice {
				found = true
				device := fmt.Sprintf("%s:%s", dpath, filepath.Join(dest, strings.TrimPrefix(dpath, src)))
				if devmode != "" {
					device = fmt.Sprintf("%s:%s", device, devmode)
				}
				if err := addDevice(g, device); err != nil {
					return fmt.Errorf("failed to add %s device: %w", dpath, err)
				}
			}
			return nil
		}); err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("no devices found in %s: %w", devicePath, unix.EINVAL)
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

func addDevice(g *generate.Generator, device string) error {
	src, dst, permissions, err := ParseDevice(device)
	if err != nil {
		return err
	}
	dev, err := util.DeviceFromPath(src)
	if err != nil {
		return fmt.Errorf("%s is not a valid device: %w", src, err)
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
func ParseDevice(device string) (string, string, string, error) {
	var src string
	var dst string
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
