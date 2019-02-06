// +build !remoteclient

package main

import "github.com/urfave/cli"

func getAppCommands() []cli.Command {
	return []cli.Command{
		attachCommand,
		commitCommand,
		buildCommand,
		createCommand,
		diffCommand,
		execCommand,
		killCommand,
		kubeCommand,
		loadCommand,
		loginCommand,
		logoutCommand,
		logsCommand,
		mountCommand,
		pauseCommand,
		psCommand,
		podCommand,
		portCommand,
		pushCommand,
		playCommand,
		restartCommand,
		rmCommand,
		runCommand,
		saveCommand,
		searchCommand,
		startCommand,
		statsCommand,
		stopCommand,
		topCommand,
		umountCommand,
		unpauseCommand,
		volumeCommand,
		waitCommand,
	}
}

func getImageSubCommands() []cli.Command {
	return []cli.Command{
		buildCommand,
		importCommand,
		loadCommand,
		pullCommand,
		saveCommand,
		trustCommand,
		signCommand,
	}
}

func getSystemSubCommands() []cli.Command {
	return []cli.Command{infoCommand}
}

func getContainerSubCommands() []cli.Command {
	return []cli.Command{
		attachCommand,
		checkpointCommand,
		cleanupCommand,
		containerExistsCommand,
		commitCommand,
		createCommand,
		diffCommand,
		execCommand,
		exportCommand,
		killCommand,
		logsCommand,
		psCommand,
		mountCommand,
		pauseCommand,
		portCommand,
		pruneContainersCommand,
		refreshCommand,
		restartCommand,
		restoreCommand,
		rmCommand,
		runCommand,
		runlabelCommand,
		startCommand,
		statsCommand,
		stopCommand,
		topCommand,
		umountCommand,
		unpauseCommand,
		//		updateCommand,
		waitCommand,
	}
}
func getMainAppFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name:  "cgroup-manager",
			Usage: "Cgroup manager to use (cgroupfs or systemd, default systemd)",
		},
		cli.StringFlag{
			Name:  "cni-config-dir",
			Usage: "Path of the configuration directory for CNI networks",
		},
		cli.StringFlag{
			Name:  "conmon",
			Usage: "Path of the conmon binary",
		},
		cli.StringFlag{
			Name:   "default-mounts-file",
			Usage:  "Path to default mounts file",
			Hidden: true,
		},
		cli.StringSliceFlag{
			Name:  "hooks-dir",
			Usage: "Set the OCI hooks directory path (may be set multiple times)",
		},
		cli.IntFlag{
			Name:   "max-workers",
			Usage:  "The maximum number of workers for parallel operations",
			Hidden: true,
		},
		cli.StringFlag{
			Name:  "namespace",
			Usage: "Set the libpod namespace, used to create separate views of the containers and pods on the system",
			Value: "",
		},
		cli.StringFlag{
			Name:  "root",
			Usage: "Path to the root directory in which data, including images, is stored",
		},
		cli.StringFlag{
			Name:  "runroot",
			Usage: "Path to the 'run directory' where all state information is stored",
		},
		cli.StringFlag{
			Name:  "runtime",
			Usage: "Path to the OCI-compatible binary used to run containers, default is /usr/bin/runc",
		},
		cli.StringFlag{
			Name:  "storage-driver, s",
			Usage: "Select which storage driver is used to manage storage of images and containers (default is overlay)",
		},
		cli.StringSliceFlag{
			Name:  "storage-opt",
			Usage: "Used to pass an option to the storage driver",
		},
		cli.BoolFlag{
			Name:  "syslog",
			Usage: "Output logging information to syslog as well as the console",
		},
	}
}
