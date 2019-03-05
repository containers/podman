// +build !remoteclient

package main

import (
	"github.com/spf13/cobra"
)

const remoteclient = false

// Commands that the local client implements
func getMainCommands() []*cobra.Command {
	rootCommands := []*cobra.Command{
		_attachCommand,
		_commitCommand,
		_createCommand,
		_diffCommand,
		_execCommand,
		_generateCommand,
		_playCommand,
		&_psCommand,
		_loginCommand,
		_logoutCommand,
		_logsCommand,
		_mountCommand,
		_pauseCommand,
		_portCommand,
		_refreshCommand,
		_restartCommand,
		_runCommand,
		_searchCommand,
		_startCommand,
		_statsCommand,
		_topCommand,
		_umountCommand,
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
		_attachCommand,
		_checkpointCommand,
		_cleanupCommand,
		_commitCommand,
		_createCommand,
		_diffCommand,
		_execCommand,
		_exportCommand,
		_killCommand,
		_logsCommand,
		_mountCommand,
		_pauseCommand,
		_portCommand,
		_pruneContainersCommand,
		_refreshCommand,
		_restartCommand,
		_restoreCommand,
		_rmCommand,
		_runCommand,
		_runlabelCommand,
		_startCommand,
		_statsCommand,
		_stopCommand,
		_topCommand,
		_umountCommand,
		_unpauseCommand,
		_waitCommand,
	}
}

func getGenerateSubCommands() []*cobra.Command {
	return []*cobra.Command{
		_containerKubeCommand,
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
	}
}

// Commands that the local client implements
func getHealtcheckSubCommands() []*cobra.Command {
	return []*cobra.Command{
		_healthcheckrunCommand,
	}
}
