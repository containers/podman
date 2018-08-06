// +build linux

package libpod

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/containerd/cgroups"
	"github.com/containers/storage/pkg/idtools"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/projectatomic/libpod/utils"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func (r *OCIRuntime) moveConmonToCgroup(ctr *Container, cgroupParent string, cmd *exec.Cmd) error {
	if os.Getuid() == 0 {
		if r.cgroupManager == SystemdCgroupsManager {
			unitName := createUnitName("libpod-conmon", ctr.ID())

			logrus.Infof("Running conmon under slice %s and unitName %s", cgroupParent, unitName)
			if err := utils.RunUnderSystemdScope(cmd.Process.Pid, cgroupParent, unitName); err != nil {
				logrus.Warnf("Failed to add conmon to systemd sandbox cgroup: %v", err)
			}
		} else {
			cgroupPath := filepath.Join(ctr.config.CgroupParent, "conmon")
			control, err := cgroups.New(cgroups.V1, cgroups.StaticPath(cgroupPath), &spec.LinuxResources{})
			if err != nil {
				logrus.Warnf("Failed to add conmon to cgroupfs sandbox cgroup: %v", err)
			} else {
				// we need to remove this defer and delete the cgroup once conmon exits
				// maybe need a conmon monitor?
				if err := control.Add(cgroups.Process{Pid: cmd.Process.Pid}); err != nil {
					logrus.Warnf("Failed to add conmon to cgroupfs sandbox cgroup: %v", err)
				}
			}
		}
	}
	return nil
}

// newPipe creates a unix socket pair for communication
func newPipe() (parent *os.File, child *os.File, err error) {
	fds, err := unix.Socketpair(unix.AF_LOCAL, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	return os.NewFile(uintptr(fds[1]), "parent"), os.NewFile(uintptr(fds[0]), "child"), nil
}

// CreateContainer creates a container in the OCI runtime
// TODO terminal support for container
// Presently just ignoring conmon opts related to it
func (r *OCIRuntime) createContainer(ctr *Container, cgroupParent string) (err error) {
	if ctr.state.UserNSRoot == "" {
		// no need of an intermediate mount ns
		return r.createOCIContainer(ctr, cgroupParent)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		runtime.LockOSThread()

		fd, err := os.Open(fmt.Sprintf("/proc/%d/task/%d/ns/mnt", os.Getpid(), unix.Gettid()))
		if err != nil {
			return
		}
		defer fd.Close()

		// create a new mountns on the current thread
		if err = unix.Unshare(unix.CLONE_NEWNS); err != nil {
			return
		}
		defer unix.Setns(int(fd.Fd()), unix.CLONE_NEWNS)

		// don't spread our mounts around
		err = unix.Mount("/", "/", "none", unix.MS_REC|unix.MS_SLAVE, "")
		if err != nil {
			return
		}
		err = unix.Mount(ctr.state.Mountpoint, ctr.state.RealMountpoint, "none", unix.MS_BIND, "")
		if err != nil {
			return
		}
		if err := idtools.MkdirAllAs(ctr.state.DestinationRunDir, 0700, ctr.RootUID(), ctr.RootGID()); err != nil {
			return
		}

		err = unix.Mount(ctr.state.RunDir, ctr.state.DestinationRunDir, "none", unix.MS_BIND, "")
		if err != nil {
			return
		}
		err = r.createOCIContainer(ctr, cgroupParent)
	}()
	wg.Wait()

	return err
}

func rpmVersion(path string) string {
	output := "Unknown"
	cmd := exec.Command("/usr/bin/rpm", "-q", "-f", path)
	if outp, err := cmd.Output(); err == nil {
		output = string(outp)
	}
	return strings.Trim(output, "\n")
}

func (r *OCIRuntime) pathPackage() string {
	return rpmVersion(r.path)
}

func (r *OCIRuntime) conmonPackage() string {
	return rpmVersion(r.conmonPath)
}
