//go:build linux
// +build linux

package cgroups

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

var (
	isUnifiedOnce sync.Once
	isUnified     bool
	isUnifiedErr  error
)

// IsCgroup2UnifiedMode returns whether we are running in cgroup 2 cgroup2 mode.
func IsCgroup2UnifiedMode() (bool, error) {
	isUnifiedOnce.Do(func() {
		var st syscall.Statfs_t
		if err := syscall.Statfs("/sys/fs/cgroup", &st); err != nil {
			isUnified, isUnifiedErr = false, err
		} else {
			isUnified, isUnifiedErr = st.Type == unix.CGROUP2_SUPER_MAGIC, nil
		}
	})
	return isUnified, isUnifiedErr
}

// UserOwnsCurrentSystemdCgroup checks whether the current EUID owns the
// current cgroup.
func UserOwnsCurrentSystemdCgroup() (bool, error) {
	uid := os.Geteuid()

	cgroup2, err := IsCgroup2UnifiedMode()
	if err != nil {
		return false, err
	}

	f, err := os.Open("/proc/self/cgroup")
	if err != nil {
		return false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 3)

		if len(parts) < 3 {
			continue
		}

		var cgroupPath string

		if cgroup2 {
			cgroupPath = filepath.Join(cgroupRoot, parts[2])
		} else {
			if parts[1] != "name=systemd" {
				continue
			}
			cgroupPath = filepath.Join(cgroupRoot, "systemd", parts[2])
		}

		st, err := os.Stat(cgroupPath)
		if err != nil {
			return false, err
		}
		s := st.Sys()
		if s == nil {
			return false, fmt.Errorf("error stat cgroup path %s", cgroupPath)
		}

		if int(s.(*syscall.Stat_t).Uid) != uid {
			return false, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, errors.Wrapf(err, "parsing file /proc/self/cgroup")
	}
	return true, nil
}

// rmDirRecursively delete recursively a cgroup directory.
// It differs from os.RemoveAll as it doesn't attempt to unlink files.
// On cgroupfs we are allowed only to rmdir empty directories.
func rmDirRecursively(path string) error {
	if err := os.Remove(path); err == nil || os.IsNotExist(err) {
		return nil
	}
	entries, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}
	for _, i := range entries {
		if i.IsDir() {
			if err := rmDirRecursively(filepath.Join(path, i.Name())); err != nil {
				return err
			}
		}
	}

	attempts := 0
	for {
		err := os.Remove(path)
		if err == nil || os.IsNotExist(err) {
			return nil
		}
		if errors.Is(err, unix.EBUSY) {
			// attempt up to 5 seconds if the cgroup is busy
			if attempts < 500 {
				time.Sleep(time.Millisecond * 10)
				attempts++
				continue
			}
		}
		return errors.Wrapf(err, "remove %s", path)
	}
}
