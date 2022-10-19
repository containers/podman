//go:build linux
// +build linux

package cgroups

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

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
			return false, fmt.Errorf("stat cgroup path %s", cgroupPath)
		}

		if int(s.(*syscall.Stat_t).Uid) != uid {
			return false, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("parsing file /proc/self/cgroup: %w", err)
	}
	return true, nil
}

// rmDirRecursively delete recursively a cgroup directory.
// It differs from os.RemoveAll as it doesn't attempt to unlink files.
// On cgroupfs we are allowed only to rmdir empty directories.
func rmDirRecursively(path string) error {
	killProcesses := func(signal syscall.Signal) {
		if signal == unix.SIGKILL {
			if err := os.WriteFile(filepath.Join(path, "cgroup.kill"), []byte("1"), 0o600); err == nil {
				return
			}
		}
		// kill all the processes that are still part of the cgroup
		if procs, err := os.ReadFile(filepath.Join(path, "cgroup.procs")); err == nil {
			for _, pidS := range strings.Split(string(procs), "\n") {
				if pid, err := strconv.Atoi(pidS); err == nil {
					_ = unix.Kill(pid, signal)
				}
			}
		}
	}

	if err := os.Remove(path); err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	entries, err := os.ReadDir(path)
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
		if err == nil || errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if errors.Is(err, unix.EBUSY) {
			// send a SIGTERM after 3 second
			if attempts == 300 {
				killProcesses(unix.SIGTERM)
			}
			// send SIGKILL after 8 seconds
			if attempts == 800 {
				killProcesses(unix.SIGKILL)
			}
			// give up after 10 seconds
			if attempts < 1000 {
				time.Sleep(time.Millisecond * 10)
				attempts++
				continue
			}
		}
		return fmt.Errorf("remove %s: %w", path, err)
	}
}
