package libpod

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/containers/podman/v4/pkg/errorhandling"
	pmount "github.com/containers/storage/pkg/mount"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func (r *ConmonOCIRuntime) createRootlessContainer(ctr *Container, restoreOptions *ContainerCheckpointOptions) (int64, error) {
	type result struct {
		restoreDuration int64
		err             error
	}
	ch := make(chan result)
	go func() {
		runtime.LockOSThread()
		restoreDuration, err := func() (int64, error) {
			fd, err := os.Open(fmt.Sprintf("/proc/%d/task/%d/ns/mnt", os.Getpid(), unix.Gettid()))
			if err != nil {
				return 0, err
			}
			defer errorhandling.CloseQuiet(fd)

			// create a new mountns on the current thread
			if err = unix.Unshare(unix.CLONE_NEWNS); err != nil {
				return 0, err
			}
			defer func() {
				if err := unix.Setns(int(fd.Fd()), unix.CLONE_NEWNS); err != nil {
					logrus.Errorf("Unable to clone new namespace: %q", err)
				}
			}()

			// don't spread our mounts around.  We are setting only /sys to be slave
			// so that the cleanup process is still able to umount the storage and the
			// changes are propagated to the host.
			err = unix.Mount("/sys", "/sys", "none", unix.MS_REC|unix.MS_SLAVE, "")
			if err != nil {
				return 0, fmt.Errorf("cannot make /sys slave: %w", err)
			}

			mounts, err := pmount.GetMounts()
			if err != nil {
				return 0, err
			}
			for _, m := range mounts {
				if !strings.HasPrefix(m.Mountpoint, "/sys/kernel") {
					continue
				}
				err = unix.Unmount(m.Mountpoint, 0)
				if err != nil && !os.IsNotExist(err) {
					return 0, fmt.Errorf("cannot unmount %s: %w", m.Mountpoint, err)
				}
			}
			return r.createOCIContainer(ctr, restoreOptions)
		}()
		ch <- result{
			restoreDuration: restoreDuration,
			err:             err,
		}
	}()
	res := <-ch
	return res.restoreDuration, res.err
}

// Run the closure with the container's socket label set
func (r *ConmonOCIRuntime) withContainerSocketLabel(ctr *Container, closure func() error) error {
	runtime.LockOSThread()
	if err := label.SetSocketLabel(ctr.ProcessLabel()); err != nil {
		return err
	}
	err := closure()
	// Ignore error returned from SetSocketLabel("") call,
	// can't recover.
	if labelErr := label.SetSocketLabel(""); labelErr == nil {
		// Unlock the thread only if the process label could be restored
		// successfully.  Otherwise leave the thread locked and the Go runtime
		// will terminate it once it returns to the threads pool.
		runtime.UnlockOSThread()
	} else {
		logrus.Errorf("Unable to reset socket label: %q", labelErr)
	}
	return err
}
