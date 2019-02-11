// +build !remoteclient

package main

import (
	"github.com/spf13/cobra"
)

// Commands that the local client implements
func getMainCommands() []*cobra.Command {
	rootCommands := []*cobra.Command{
		_attachCommand,
		_buildCommand,
		_commitCommand,
		_createCommand,
		_diffCommand,
		_execCommand,
		_killCommand,
		generateCommand.Command,
		podCommand.Command,
		_containerKubeCommand,
		_psCommand,
		_loadCommand,
		_loginCommand,
		_logoutCommand,
		_logsCommand,
		_mountCommand,
		_pauseCommand,
		_portCommand,
		_pushCommand,
		_refreshCommand,
		_restartCommand,
		_restoreCommand,
		_rmCommand,
		_runCommmand,
		_saveCommand,
		_searchCommand,
		_signCommand,
		_startCommand,
		_statsCommand,
		_stopCommand,
		_topCommand,
		_umountCommand,
		_unpauseCommand,
		_varlinkCommand,
		volumeCommand.Command,
		_waitCommand,
	}
	return rootCommands
}

// Commands that the local client implements
func getImageSubCommands() []*cobra.Command {
	return []*cobra.Command{
		_buildCommand,
		_loadCommand,
		_pushCommand,
		_saveCommand,
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
		_psCommand,
		_mountCommand,
		_pauseCommand,
		_portCommand,
		_pruneContainersCommand,
		_refreshCommand,
		_restartCommand,
		_restoreCommand,
		_rmCommand,
		_runCommmand,
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

// Commands that the local client implements
func getPodSubCommands() []*cobra.Command {
	return []*cobra.Command{
		_podCreateCommand,
		_podExistsCommand,
		_podInspectCommand,
		_podKillCommand,
		_podPauseCommand,
		_podPsCommand,
		_podRestartCommand,
		_podRmCommand,
		_podStartCommand,
		_podStatsCommand,
		_podStopCommand,
		_podTopCommand,
		_podUnpauseCommand,
	}
}

// Commands that the local client implements
func getVolumeSubCommands() []*cobra.Command {
	return []*cobra.Command{
		_volumeCreateCommand,
		_volumeLsCommand,
		_volumeRmCommand,
		_volumeInspectCommand,
		_volumePruneCommand,
	}
}

// Commands that the local client implements
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
	}
}
