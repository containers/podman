// Copyright 2018 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This file was originally a part of the containernetworking/plugins
// repository.
// It was copied here and modified for local use by the libpod maintainers.

package netns

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containers/common/pkg/util"
	"github.com/containers/storage/pkg/unshare"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// GetNSRunDir returns the dir of where to create the netNS. When running
// rootless, it needs to be at a location writable by user.
func GetNSRunDir() (string, error) {
	if unshare.IsRootless() {
		rootlessDir, err := util.GetRuntimeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(rootlessDir, "netns"), nil
	}
	return "/run/netns", nil
}

// NewNS creates a new persistent (bind-mounted) network namespace and returns
// an object representing that namespace, without switching to it.
func NewNS() (ns.NetNS, error) {
	for i := 0; i < 10000; i++ {
		b := make([]byte, 16)
		_, err := rand.Reader.Read(b)
		if err != nil {
			return nil, fmt.Errorf("failed to generate random netns name: %v", err)
		}
		nsName := fmt.Sprintf("netns-%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
		ns, err := NewNSWithName(nsName)
		if err == nil {
			return ns, nil
		}
		// retry when the name already exists
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return nil, err
	}
	return nil, errors.New("failed to find free netns path name")
}

// NewNSWithName creates a new persistent (bind-mounted) network namespace and returns
// an object representing that namespace, without switching to it.
func NewNSWithName(name string) (ns.NetNS, error) {
	nsRunDir, err := GetNSRunDir()
	if err != nil {
		return nil, err
	}

	// Create the directory for mounting network namespaces
	// This needs to be a shared mountpoint in case it is mounted in to
	// other namespaces (containers)
	err = os.MkdirAll(nsRunDir, 0o755)
	if err != nil {
		return nil, err
	}

	// Remount the namespace directory shared. This will fail if it is not
	// already a mountpoint, so bind-mount it on to itself to "upgrade" it
	// to a mountpoint.
	err = unix.Mount("", nsRunDir, "none", unix.MS_SHARED|unix.MS_REC, "")
	if err != nil {
		if err != unix.EINVAL {
			return nil, fmt.Errorf("mount --make-rshared %s failed: %q", nsRunDir, err)
		}

		// Recursively remount /run/netns on itself. The recursive flag is
		// so that any existing netns bindmounts are carried over.
		err = unix.Mount(nsRunDir, nsRunDir, "none", unix.MS_BIND|unix.MS_REC, "")
		if err != nil {
			return nil, fmt.Errorf("mount --rbind %s %s failed: %q", nsRunDir, nsRunDir, err)
		}

		// Now we can make it shared
		err = unix.Mount("", nsRunDir, "none", unix.MS_SHARED|unix.MS_REC, "")
		if err != nil {
			return nil, fmt.Errorf("mount --make-rshared %s failed: %q", nsRunDir, err)
		}
	}

	// create an empty file at the mount point
	nsPath := path.Join(nsRunDir, name)
	mountPointFd, err := os.OpenFile(nsPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	if err := mountPointFd.Close(); err != nil {
		return nil, err
	}

	// Ensure the mount point is cleaned up on errors; if the namespace
	// was successfully mounted this will have no effect because the file
	// is in-use
	defer func() {
		_ = os.RemoveAll(nsPath)
	}()

	var wg sync.WaitGroup
	wg.Add(1)

	// do namespace work in a dedicated goroutine, so that we can safely
	// Lock/Unlock OSThread without upsetting the lock/unlock state of
	// the caller of this function
	go (func() {
		defer wg.Done()
		runtime.LockOSThread()
		// Don't unlock. By not unlocking, golang will kill the OS thread when the
		// goroutine is done (for go1.10+)

		threadNsPath := getCurrentThreadNetNSPath()

		var origNS ns.NetNS
		origNS, err = ns.GetNS(threadNsPath)
		if err != nil {
			logrus.Warnf("Cannot open current network namespace %s: %q", threadNsPath, err)
			return
		}
		defer func() {
			if err := origNS.Close(); err != nil {
				logrus.Errorf("Unable to close namespace: %q", err)
			}
		}()

		// create a new netns on the current thread
		err = unix.Unshare(unix.CLONE_NEWNET)
		if err != nil {
			logrus.Warnf("Cannot create a new network namespace: %q", err)
			return
		}

		// bind mount the netns from the current thread (from /proc) onto the
		// mount point. This causes the namespace to persist, even when there
		// are no threads in the ns. Make this a shared mount; it needs to be
		// back-propagated to the host
		err = unix.Mount(threadNsPath, nsPath, "none", unix.MS_BIND|unix.MS_SHARED|unix.MS_REC, "")
		if err != nil {
			err = fmt.Errorf("failed to bind mount ns at %s: %v", nsPath, err)
		}
	})()
	wg.Wait()

	if err != nil {
		return nil, fmt.Errorf("failed to create namespace: %v", err)
	}

	return ns.GetNS(nsPath)
}

// UnmountNS unmounts the given netns path
func UnmountNS(nsPath string) error {
	nsRunDir, err := GetNSRunDir()
	if err != nil {
		return err
	}

	// Only unmount if it's been bind-mounted (don't touch namespaces in /proc...)
	if strings.HasPrefix(nsPath, nsRunDir) {
		if err := unix.Unmount(nsPath, unix.MNT_DETACH); err != nil {
			return fmt.Errorf("failed to unmount NS: at %s: %v", nsPath, err)
		}

		if err := os.Remove(nsPath); err != nil {
			return fmt.Errorf("failed to remove ns path %s: %v", nsPath, err)
		}
	}

	return nil
}

// getCurrentThreadNetNSPath copied from pkg/ns
func getCurrentThreadNetNSPath() string {
	// /proc/self/ns/net returns the namespace of the main thread, not
	// of whatever thread this goroutine is running on.  Make sure we
	// use the thread's net namespace since the thread is switching around
	return fmt.Sprintf("/proc/%d/task/%d/ns/net", os.Getpid(), unix.Gettid())
}
