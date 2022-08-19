package libpod

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/containers/common/pkg/apparmor"
	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/common/pkg/seccomp"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/sirupsen/logrus"
)

func (r *Runtime) setPlatformHostInfo(info *define.HostInfo) error {
	seccompProfilePath, err := DefaultSeccompPath()
	if err != nil {
		return fmt.Errorf("error getting Seccomp profile path: %w", err)
	}

	// Cgroups version
	unified, err := cgroups.IsCgroup2UnifiedMode()
	if err != nil {
		return fmt.Errorf("error reading cgroups mode: %w", err)
	}

	// Get Map of all available controllers
	availableControllers, err := cgroups.GetAvailableControllers(nil, unified)
	if err != nil {
		return fmt.Errorf("error getting available cgroup controllers: %w", err)
	}

	info.CgroupManager = r.config.Engine.CgroupManager
	info.CgroupControllers = availableControllers
	info.IDMappings = define.IDMappings{}
	info.Security = define.SecurityInfo{
		AppArmorEnabled:     apparmor.IsEnabled(),
		DefaultCapabilities: strings.Join(r.config.Containers.DefaultCapabilities, ","),
		Rootless:            rootless.IsRootless(),
		SECCOMPEnabled:      seccomp.IsEnabled(),
		SECCOMPProfilePath:  seccompProfilePath,
		SELinuxEnabled:      selinux.GetEnabled(),
	}
	info.Slirp4NetNS = define.SlirpInfo{}

	cgroupVersion := "v1"
	if unified {
		cgroupVersion = "v2"
	}
	info.CgroupsVersion = cgroupVersion

	slirp4netnsPath := r.config.Engine.NetworkCmdPath
	if slirp4netnsPath == "" {
		slirp4netnsPath, _ = exec.LookPath("slirp4netns")
	}
	if slirp4netnsPath != "" {
		version, err := programVersion(slirp4netnsPath)
		if err != nil {
			logrus.Warnf("Failed to retrieve program version for %s: %v", slirp4netnsPath, err)
		}
		program := define.SlirpInfo{
			Executable: slirp4netnsPath,
			Package:    packageVersion(slirp4netnsPath),
			Version:    version,
		}
		info.Slirp4NetNS = program
	}

	if rootless.IsRootless() {
		uidmappings, err := rootless.ReadMappingsProc("/proc/self/uid_map")
		if err != nil {
			return fmt.Errorf("error reading uid mappings: %w", err)
		}
		gidmappings, err := rootless.ReadMappingsProc("/proc/self/gid_map")
		if err != nil {
			return fmt.Errorf("error reading gid mappings: %w", err)
		}
		idmappings := define.IDMappings{
			GIDMap: gidmappings,
			UIDMap: uidmappings,
		}
		info.IDMappings = idmappings
	}

	return nil
}
