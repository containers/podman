package util

import (
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/psgo"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

var (
	errNotADevice = errors.New("not a device node")
)

// GetContainerPidInformationDescriptors returns a string slice of all supported
// format descriptors of GetContainerPidInformation.
func GetContainerPidInformationDescriptors() ([]string, error) {
	return psgo.ListDescriptors(), nil
}

// FindDeviceNodes parses /dev/ into a set of major:minor -> path, where
// [major:minor] is the device's major and minor numbers formatted as, for
// example, 2:0 and path is the path to the device node.
// Symlinks to nodes are ignored.
func FindDeviceNodes() (map[string]string, error) {
	nodes := make(map[string]string)
	err := filepath.WalkDir("/dev", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			logrus.Warnf("Error descending into path %s: %v", path, err)
			return filepath.SkipDir
		}

		// If we aren't a device node, do nothing.
		if d.Type()&(os.ModeDevice|os.ModeCharDevice) == 0 {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		// We are a device node. Get major/minor.
		sysstat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return errors.New("could not convert stat output for use")
		}
		// We must typeconvert sysstat.Rdev from uint64->int to avoid constant overflow
		rdev := int(sysstat.Rdev)
		major := ((rdev >> 8) & 0xfff) | ((rdev >> 32) & ^0xfff)
		minor := (rdev & 0xff) | ((rdev >> 12) & ^0xff)

		nodes[fmt.Sprintf("%d:%d", major, minor)] = path

		return nil
	})
	if err != nil {
		return nil, err
	}

	return nodes, nil
}

func AddPrivilegedDevices(g *generate.Generator) error {
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

		device, err := DeviceFromPath(filepath.Join(path, f.Name()))
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

// Copied from github.com/opencontainers/runc/libcontainer/devices
// Given the path to a device look up the information about a linux device
func DeviceFromPath(path string) (*spec.LinuxDevice, error) {
	var stat unix.Stat_t
	err := unix.Lstat(path, &stat)
	if err != nil {
		return nil, err
	}
	var (
		devType   string
		mode      = stat.Mode
		devNumber = uint64(stat.Rdev) //nolint: unconvert
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
