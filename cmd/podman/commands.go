// +build !remoteclient

package main

import (
	"fmt"

	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/libpod/pkg/apparmor"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/spf13/cobra"
)

const remoteclient = false

// Commands that the local client implements
func getMainCommands() []*cobra.Command {
	rootCommands := []*cobra.Command{
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
	if len(_serviceCommand.Use) > 0 {
		rootCommands = append(rootCommands, _serviceCommand)
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
	return []*cobra.Command{
		_renumberCommand,
		_dfSystemCommand,
		_migrateCommand,
	}
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
