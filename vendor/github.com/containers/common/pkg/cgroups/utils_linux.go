//go:build linux
// +build linux

package cgroups

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// WriteFile writes to a cgroup file
func WriteFile(dir, file, data string) error {
	fd, err := OpenFile(dir, file, unix.O_WRONLY)
	if err != nil {
		return err
	}
	defer fd.Close()
	for {
		_, err := fd.Write([]byte(data))
		if errors.Is(err, unix.EINTR) {
			logrus.Infof("interrupted while writing %s to %s", data, fd.Name())
			continue
		}
		return err
	}
}

// OpenFile opens a cgroup file with the given flags
func OpenFile(dir, file string, flags int) (*os.File, error) {
	var resolveFlags uint64
	mode := os.FileMode(0)
	if TestMode && flags&os.O_WRONLY != 0 {
		flags |= os.O_TRUNC | os.O_CREATE
		mode = 0o600
	}
	cgroupPath := path.Join(dir, file)
	relPath := strings.TrimPrefix(cgroupPath, cgroupRoot+"/")

	var stats unix.Statfs_t
	fdTest, errOpen := unix.Openat2(-1, cgroupRoot, &unix.OpenHow{
		Flags: unix.O_DIRECTORY | unix.O_PATH,
	})
	errStat := unix.Fstatfs(fdTest, &stats)
	cgroupFd := fdTest

	resolveFlags = unix.RESOLVE_BENEATH | unix.RESOLVE_NO_MAGICLINKS
	if stats.Type == unix.CGROUP2_SUPER_MAGIC {
		// cgroupv2 has a single mountpoint and no "cpu,cpuacct" symlinks
		resolveFlags |= unix.RESOLVE_NO_XDEV | unix.RESOLVE_NO_SYMLINKS
	}

	if errOpen != nil || errStat != nil || (len(relPath) == len(cgroupPath)) { // openat2 not available, use os
		fdTest, err := os.OpenFile(cgroupPath, flags, mode)
		if err != nil {
			return nil, err
		}
		if TestMode {
			return fdTest, nil
		}
		if err := unix.Fstatfs(int(fdTest.Fd()), &stats); err != nil {
			_ = fdTest.Close()
			return nil, &os.PathError{Op: "statfs", Path: cgroupPath, Err: err}
		}
		if stats.Type != unix.CGROUP_SUPER_MAGIC && stats.Type != unix.CGROUP2_SUPER_MAGIC {
			_ = fdTest.Close()
			return nil, &os.PathError{Op: "open", Path: cgroupPath, Err: errors.New("not a cgroup file")}
		}
		return fdTest, nil
	}

	fd, err := unix.Openat2(cgroupFd, relPath,
		&unix.OpenHow{
			Resolve: resolveFlags,
			Flags:   uint64(flags) | unix.O_CLOEXEC,
			Mode:    uint64(mode),
		})
	if err != nil {
		fmt.Println("Error in openat")
		return nil, err
	}

	return os.NewFile(uintptr(fd), cgroupPath), nil
}

// ReadFile reads from a cgroup file, opening it with the read only flag
func ReadFile(dir, file string) (string, error) {
	fd, err := OpenFile(dir, file, unix.O_RDONLY)
	if err != nil {
		return "", err
	}
	defer fd.Close()
	var buf bytes.Buffer

	_, err = buf.ReadFrom(fd)
	return buf.String(), err
}

// GetBlkioFiles gets the proper files for blkio weights
func GetBlkioFiles(cgroupPath string) (wtFile, wtDevFile string) {
	var weightFile string
	var weightDeviceFile string
	// in this important since runc keeps these variables private, they won't be set
	if cgroups.PathExists(filepath.Join(cgroupPath, "blkio.weight")) {
		weightFile = "blkio.weight"
		weightDeviceFile = "blkio.weight_device"
	} else {
		weightFile = "blkio.bfq.weight"
		weightDeviceFile = "blkio.bfq.weight_device"
	}
	return weightFile, weightDeviceFile
}

// SetBlkioThrottle sets the throttle limits for the cgroup
func SetBlkioThrottle(res *configs.Resources, cgroupPath string) error {
	for _, td := range res.BlkioThrottleReadBpsDevice {
		if err := WriteFile(cgroupPath, "blkio.throttle.read_bps_device", fmt.Sprintf("%d:%d %d", td.Major, td.Minor, td.Rate)); err != nil {
			return err
		}
	}
	for _, td := range res.BlkioThrottleWriteBpsDevice {
		if err := WriteFile(cgroupPath, "blkio.throttle.write_bps_device", fmt.Sprintf("%d:%d %d", td.Major, td.Minor, td.Rate)); err != nil {
			return err
		}
	}
	for _, td := range res.BlkioThrottleReadIOPSDevice {
		if err := WriteFile(cgroupPath, "blkio.throttle.read_iops_device", td.String()); err != nil {
			return err
		}
	}
	for _, td := range res.BlkioThrottleWriteIOPSDevice {
		if err := WriteFile(cgroupPath, "blkio.throttle.write_iops_device", td.String()); err != nil {
			return err
		}
	}
	return nil
}
