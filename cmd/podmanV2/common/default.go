package common

import (
	"fmt"
	"os"

	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/libpod/pkg/apparmor"
	"github.com/containers/libpod/pkg/cgroups"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/sysinfo"
	"github.com/opencontainers/selinux/go-selinux"
)

// TODO these options are directly embedded into many of the CLI cobra values, as such
// this approach will not work in a remote client. so we will need to likely do something like a
// supported and unsupported approach here and backload these options into the specgen
// once we are "on" the host system.
func getDefaultSecurityOptions() []string {
	securityOpts := []string{}
	if defaultContainerConfig.Containers.SeccompProfile != "" && defaultContainerConfig.Containers.SeccompProfile != parse.SeccompDefaultPath {
		securityOpts = append(securityOpts, fmt.Sprintf("seccomp=%s", defaultContainerConfig.Containers.SeccompProfile))
	}
	if apparmor.IsEnabled() && defaultContainerConfig.Containers.ApparmorProfile != "" {
		securityOpts = append(securityOpts, fmt.Sprintf("apparmor=%s", defaultContainerConfig.Containers.ApparmorProfile))
	}
	if selinux.GetEnabled() && !defaultContainerConfig.Containers.EnableLabeling {
		securityOpts = append(securityOpts, fmt.Sprintf("label=%s", selinux.DisableSecOpt()[0]))
	}
	return securityOpts
}

// getDefaultSysctls
func getDefaultSysctls() []string {
	return defaultContainerConfig.Containers.DefaultSysctls
}

func getDefaultVolumes() []string {
	return defaultContainerConfig.Containers.Volumes
}

func getDefaultDevices() []string {
	return defaultContainerConfig.Containers.Devices
}

func getDefaultDNSServers() []string { //nolint
	return defaultContainerConfig.Containers.DNSServers
}

func getDefaultDNSSearches() []string { //nolint
	return defaultContainerConfig.Containers.DNSSearches
}

func getDefaultDNSOptions() []string { //nolint
	return defaultContainerConfig.Containers.DNSOptions
}

func getDefaultEnv() []string {
	return defaultContainerConfig.Containers.Env
}

func getDefaultInitPath() string {
	return defaultContainerConfig.Containers.InitPath
}

func getDefaultIPCNS() string {
	return defaultContainerConfig.Containers.IPCNS
}

func getDefaultPidNS() string {
	return defaultContainerConfig.Containers.PidNS
}

func getDefaultNetNS() string { //nolint
	if defaultContainerConfig.Containers.NetNS == "private" && rootless.IsRootless() {
		return "slirp4netns"
	}
	return defaultContainerConfig.Containers.NetNS
}

func getDefaultCgroupNS() string {
	return defaultContainerConfig.Containers.CgroupNS
}

func getDefaultUTSNS() string {
	return defaultContainerConfig.Containers.UTSNS
}

func getDefaultShmSize() string {
	return defaultContainerConfig.Containers.ShmSize
}

func getDefaultUlimits() []string {
	return defaultContainerConfig.Containers.DefaultUlimits
}

func getDefaultUserNS() string {
	userns := os.Getenv("PODMAN_USERNS")
	if userns != "" {
		return userns
	}
	return defaultContainerConfig.Containers.UserNS
}

func getDefaultPidsLimit() int64 {
	if rootless.IsRootless() {
		cgroup2, _ := cgroups.IsCgroup2UnifiedMode()
		if cgroup2 {
			return defaultContainerConfig.Containers.PidsLimit
		}
	}
	return sysinfo.GetDefaultPidsLimit()
}

func getDefaultPidsDescription() string {
	return "Tune container pids limit (set 0 for unlimited)"
}

func GetDefaultDetachKeys() string {
	return defaultContainerConfig.Engine.DetachKeys
}
