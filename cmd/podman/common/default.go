package common

import (
	"fmt"
	"os"

	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/libpod/pkg/apparmor"
	"github.com/containers/libpod/pkg/cgroups"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/specgen"
	"github.com/containers/libpod/pkg/sysinfo"
	"github.com/opencontainers/selinux/go-selinux"
)

var (
	// DefaultHealthCheckInterval default value
	DefaultHealthCheckInterval = "30s"
	// DefaultHealthCheckRetries default value
	DefaultHealthCheckRetries uint = 3
	// DefaultHealthCheckStartPeriod default value
	DefaultHealthCheckStartPeriod = "0s"
	// DefaultHealthCheckTimeout default value
	DefaultHealthCheckTimeout = "30s"
	// DefaultImageVolume default value
	DefaultImageVolume = "bind"
)

// TODO these options are directly embedded into many of the CLI cobra values, as such
// this approach will not work in a remote client. so we will need to likely do something like a
// supported and unsupported approach here and backload these options into the specgen
// once we are "on" the host system.
func getDefaultSecurityOptions() []string {
	securityOpts := []string{}
	if containerConfig.Containers.SeccompProfile != "" && containerConfig.Containers.SeccompProfile != parse.SeccompDefaultPath {
		securityOpts = append(securityOpts, fmt.Sprintf("seccomp=%s", containerConfig.Containers.SeccompProfile))
	}
	if apparmor.IsEnabled() && containerConfig.Containers.ApparmorProfile != "" {
		securityOpts = append(securityOpts, fmt.Sprintf("apparmor=%s", containerConfig.Containers.ApparmorProfile))
	}
	if selinux.GetEnabled() && !containerConfig.Containers.EnableLabeling {
		securityOpts = append(securityOpts, fmt.Sprintf("label=%s", selinux.DisableSecOpt()[0]))
	}
	return securityOpts
}

// getDefaultSysctls
func getDefaultSysctls() []string {
	return containerConfig.Containers.DefaultSysctls
}

func getDefaultVolumes() []string {
	return containerConfig.Containers.Volumes
}

func getDefaultDevices() []string {
	return containerConfig.Containers.Devices
}

func getDefaultDNSServers() []string { //nolint
	return containerConfig.Containers.DNSServers
}

func getDefaultDNSSearches() []string { //nolint
	return containerConfig.Containers.DNSSearches
}

func getDefaultDNSOptions() []string { //nolint
	return containerConfig.Containers.DNSOptions
}

func getDefaultEnv() []string {
	return containerConfig.Containers.Env
}

func getDefaultInitPath() string {
	return containerConfig.Containers.InitPath
}

func getDefaultIPCNS() string {
	return containerConfig.Containers.IPCNS
}

func getDefaultPidNS() string {
	return containerConfig.Containers.PidNS
}

func getDefaultNetNS() string { //nolint
	if containerConfig.Containers.NetNS == string(specgen.Private) && rootless.IsRootless() {
		return string(specgen.Slirp)
	}
	return containerConfig.Containers.NetNS
}

func getDefaultCgroupNS() string {
	return containerConfig.Containers.CgroupNS
}

func getDefaultUTSNS() string {
	return containerConfig.Containers.UTSNS
}

func getDefaultShmSize() string {
	return containerConfig.Containers.ShmSize
}

func getDefaultUlimits() []string {
	return containerConfig.Containers.DefaultUlimits
}

func getDefaultUserNS() string {
	userns := os.Getenv("PODMAN_USERNS")
	if userns != "" {
		return userns
	}
	return containerConfig.Containers.UserNS
}

func getDefaultPidsLimit() int64 {
	if rootless.IsRootless() {
		cgroup2, _ := cgroups.IsCgroup2UnifiedMode()
		if cgroup2 {
			return containerConfig.Containers.PidsLimit
		}
	}
	return sysinfo.GetDefaultPidsLimit()
}

func getDefaultPidsDescription() string {
	return "Tune container pids limit (set 0 for unlimited)"
}

func GetDefaultDetachKeys() string {
	return containerConfig.Engine.DetachKeys
}
