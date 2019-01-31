// +build !remoteclient

package main

import (
	"github.com/spf13/cobra"
)

func getImageSubCommands() []*cobra.Command {
	return []*cobra.Command{
		_buildCommand,
		_importCommand,
		_loadCommand,
		_pullCommand,
		_rmiCommand,
		_saveCommand,
		_signCommand,
	}
}

func getContainerSubCommands() []*cobra.Command {
	return []*cobra.Command{
		_attachCommand,
		_checkpointCommand,
		_cleanupCommand,
		_containerExistsCommand,
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

func getVolumeSubCommands() []*cobra.Command {
	return []*cobra.Command{
		_volumeCreateCommand,
		_volumeLsCommand,
		_volumeRmCommand,
		_volumeInspectCommand,
		_volumePruneCommand,
	}
}

func getGenerateSubCommands() []*cobra.Command {
	return []*cobra.Command{
		_containerKubeCommand,
	}
}

func getPlaySubCommands() []*cobra.Command {
	return []*cobra.Command{
		_playKubeCommand,
	}
}

func getTrustSubCommands() []*cobra.Command {
	return []*cobra.Command{
		_setTrustCommand,
		_showTrustCommand,
	}
}

func getSystemSubCommands() []*cobra.Command {
	return []*cobra.Command{
		_infoCommand,
		_pruneSystemCommand,
	}
}
