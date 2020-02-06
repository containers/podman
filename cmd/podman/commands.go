// +build !remoteclient

package main

import (
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
