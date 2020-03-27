// +build !remoteclient

package main

import (
	"fmt"
	"os"

	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/libpod/pkg/apparmor"
	"github.com/containers/libpod/pkg/cgroups"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/sysinfo"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/spf13/cobra"
)

const remoteclient = false

// Commands that the local client implements
func getMainCommands() []*cobra.Command {
	rootCommands := []*cobra.Command{
		_autoUpdateCommand,
		_cpCommand,
		_playCommand,
		_loginCommand,
		_logoutCommand,
		_mountCommand,
		_refreshCommand,
		_searchCommand,
		_statsCommand,
		_umountCommand,
		_unshareCommand,
	}

	if len(_varlinkCommand.Use) > 0 {
		rootCommands = append(rootCommands, _varlinkCommand)
	}
	return rootCommands
}

// Commands that the local client implements
func getImageSubCommands() []*cobra.Command {
	return []*cobra.Command{
		_signCommand,
		_trustCommand,
	}
}

// Commands that the local client implements
func getContainerSubCommands() []*cobra.Command {

	return []*cobra.Command{
		_cpCommand,
		_cleanupCommand,
		_mountCommand,
		_refreshCommand,
		_runlabelCommand,
		_statsCommand,
		_umountCommand,
	}
}

// Commands that the local client implements
func getPlaySubCommands() []*cobra.Command {
	return []*cobra.Command{
		_playKubeCommand,
	}
}

// Commands that the local client implements
func getTrustSubCommands() []*cobra.Command {
	return []*cobra.Command{
		_setTrustCommand,
		_showTrustCommand,
	}
}

// Commands that the local client implements
func getSystemSubCommands() []*cobra.Command {
	systemCommands := []*cobra.Command{
		_renumberCommand,
		_dfSystemCommand,
		_migrateCommand,
	}

	if len(_serviceCommand.Use) > 0 {
		systemCommands = append(systemCommands, _serviceCommand)
	}

	return systemCommands
}

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

func getDefaultDNSServers() []string {
	return defaultContainerConfig.Containers.DNSServers
}

func getDefaultDNSSearches() []string {
	return defaultContainerConfig.Containers.DNSSearches
}

func getDefaultDNSOptions() []string {
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

func getDefaultNetNS() string {
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

func getDefaultDetachKeys() string {
	return defaultContainerConfig.Engine.DetachKeys
}
