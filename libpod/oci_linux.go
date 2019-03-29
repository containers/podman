// +build linux

package libpod

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/containerd/cgroups"
	"github.com/containers/libpod/utils"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const unknownPackage = "Unknown"

func (r *OCIRuntime) moveConmonToCgroup(ctr *Container, cgroupParent string, cmd *exec.Cmd) error {
	if os.Geteuid() == 0 {
		if r.cgroupManager == SystemdCgroupsManager {
			unitName := createUnitName("libpod-conmon", ctr.ID())

			realCgroupParent := cgroupParent
			splitParent := strings.Split(cgroupParent, "/")
			if strings.HasSuffix(cgroupParent, ".slice") && len(splitParent) > 1 {
				realCgroupParent = splitParent[len(splitParent)-1]
			}

			logrus.Infof("Running conmon under slice %s and unitName %s", realCgroupParent, unitName)
			if err := utils.RunUnderSystemdScope(cmd.Process.Pid, realCgroupParent, unitName); err != nil {
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

// makeAccessible changes the path permission and each parent directory to have --x--x--x
func makeAccessible(path string, uid, gid int) error {
	for ; path != "/"; path = filepath.Dir(path) {
		st, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if int(st.Sys().(*syscall.Stat_t).Uid) == uid && int(st.Sys().(*syscall.Stat_t).Gid) == gid {
			continue
		}
		if st.Mode()&0111 != 0111 {
			if err := os.Chmod(path, os.FileMode(st.Mode()|0111)); err != nil {
				return err
			}
		}
	}
	return nil
}

// CreateContainer creates a container in the OCI runtime
// TODO terminal support for container
// Presently just ignoring conmon opts related to it
func (r *OCIRuntime) createContainer(ctr *Container, cgroupParent string, restoreOptions *ContainerCheckpointOptions) (err error) {
	if len(ctr.config.IDMappings.UIDMap) != 0 || len(ctr.config.IDMappings.GIDMap) != 0 {
		for _, i := range []string{ctr.state.RunDir, ctr.runtime.config.TmpDir, ctr.config.StaticDir, ctr.state.Mountpoint, ctr.runtime.config.VolumePath} {
			if err := makeAccessible(i, ctr.RootUID(), ctr.RootGID()); err != nil {
				return err
			}
		}
	}
	return r.createOCIContainer(ctr, cgroupParent, restoreOptions)
}

func rpmVersion(path string) string {
	output := unknownPackage
	cmd := exec.Command("/usr/bin/rpm", "-q", "-f", path)
	if outp, err := cmd.Output(); err == nil {
		output = string(outp)
	}
	return strings.Trim(output, "\n")
}

func dpkgVersion(path string) string {
	output := unknownPackage
	cmd := exec.Command("/usr/bin/dpkg", "-S", path)
	if outp, err := cmd.Output(); err == nil {
		output = string(outp)
	}
	return strings.Trim(output, "\n")
}

func (r *OCIRuntime) pathPackage() string {
	if out := rpmVersion(r.path); out != unknownPackage {
		return out
	}
	return dpkgVersion(r.path)
}

func (r *OCIRuntime) conmonPackage() string {
	if out := rpmVersion(r.conmonPath); out != unknownPackage {
		return out
	}
	return dpkgVersion(r.conmonPath)
}
