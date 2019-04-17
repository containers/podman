// +build !remoteclient

package main

import (
	"github.com/spf13/cobra"
)

const remoteclient = false

// Commands that the local client implements
func getMainCommands() []*cobra.Command {
	rootCommands := []*cobra.Command{
		_commitCommand,
		_execCommand,
		_playCommand,
		_loginCommand,
		_logoutCommand,
		_mountCommand,
		_pauseCommand,
		_portCommand,
		_refreshCommand,
		_restartCommand,
		_searchCommand,
		_statsCommand,
		_topCommand,
		_unpauseCommand,
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
	}
}

// Commands that the local client implements
func getContainerSubCommands() []*cobra.Command {

	return []*cobra.Command{
		_checkpointCommand,
		_cleanupCommand,
		_commitCommand,
		_execCommand,
		_mountCommand,
		_pauseCommand,
		_portCommand,
		_pruneContainersCommand,
		_refreshCommand,
		_restartCommand,
		_restoreCommand,
		_runlabelCommand,
		_statsCommand,
		_stopCommand,
		_topCommand,
		_umountCommand,
		_unpauseCommand,
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
		_pruneSystemCommand,
		_renumberCommand,
		_dfSystemCommand,
	}
}

// Commands that the local client implements
func getHealthcheckSubCommands() []*cobra.Command {
	return []*cobra.Command{
		_healthcheckrunCommand,
	}
}
