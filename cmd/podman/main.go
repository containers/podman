package main

import (
	"fmt"
	"os"
	"runtime/pprof"

	"github.com/containers/storage/pkg/reexec"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// This is populated by the Makefile from the VERSION file
// in the repository
var (
	podmanVersion = ""
	exitCode      = 125
)

func main() {
	debug := false
	cpuProfile := false

	if reexec.Init() {
		return
	}

	app := cli.NewApp()
	app.Name = "podman"
	app.Usage = "manage pods and images"

	var v string
	if podmanVersion != "" {
		v = podmanVersion
	}
	app.Version = v

	app.Commands = []cli.Command{
		attachCommand,
		commitCommand,
		buildCommand,
		createCommand,
		diffCommand,
		execCommand,
		exportCommand,
		historyCommand,
		imagesCommand,
		importCommand,
		infoCommand,
		inspectCommand,
		killCommand,
		loadCommand,
		loginCommand,
		logoutCommand,
		logsCommand,
		mountCommand,
		pauseCommand,
		psCommand,
		portCommand,
		pullCommand,
		pushCommand,
		rmCommand,
		rmiCommand,
		runCommand,
		saveCommand,
		searchCommand,
		startCommand,
		statsCommand,
		stopCommand,
		tagCommand,
		topCommand,
		umountCommand,
		unpauseCommand,
		versionCommand,
		waitCommand,
	}
	app.Before = func(c *cli.Context) error {
		logLevel := c.GlobalString("log-level")
		if logLevel != "" {
			level, err := logrus.ParseLevel(logLevel)
			if err != nil {
				return err
			}

			logrus.SetLevel(level)
		}

		if logLevel == "debug" {
			debug = true

		}
		if c.GlobalIsSet("cpu-profile") {
			f, err := os.Create(c.GlobalString("cpu-profile"))
			if err != nil {
				return errors.Wrapf(err, "unable to create cpu profiling file %s",
					c.GlobalString("cpu-profile"))
			}
			cpuProfile = true
			pprof.StartCPUProfile(f)
		}
		return nil
	}
	app.After = func(*cli.Context) error {
		// called by Run() when the command handler succeeds
		if cpuProfile {
			pprof.StopCPUProfile()
		}
		return nil
	}
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "cni-config-dir",
			Usage: "path of the configuration directory for CNI networks",
		},
		cli.StringFlag{
			Name:  "config, c",
			Usage: "path of a config file detailing container server configuration options",
		},
		cli.StringFlag{
			Name:  "conmon",
			Usage: "path of the conmon binary",
		},
		cli.StringFlag{
			Name:  "cpu-profile",
			Usage: "path for the cpu profiling results",
		},
		cli.StringFlag{
			Name:  "log-level",
			Usage: "log messages above specified level: debug, info, warn, error (default), fatal or panic",
			Value: "error",
		},
		cli.StringFlag{
			Name:  "root",
			Usage: "path to the root directory in which data, including images, is stored",
		},
		cli.StringFlag{
			Name:  "runroot",
			Usage: "path to the 'run directory' where all state information is stored",
		},
		cli.StringFlag{
			Name:  "runtime",
			Usage: "path to the OCI-compatible binary used to run containers, default is /usr/bin/runc",
		},
		cli.StringFlag{
			Name:  "storage-driver, s",
			Usage: "select which storage driver is used to manage storage of images and containers (default is overlay)",
		},
		cli.StringSliceFlag{
			Name:  "storage-opt",
			Usage: "used to pass an option to the storage driver",
		},
	}
	if _, err := os.Stat("/etc/containers/registries.conf"); err != nil {
		if os.IsNotExist(err) {
			logrus.Warn("unable to find /etc/containers/registries.conf. some podman (image shortnames) commands may be limited")
		}
	}
	if err := app.Run(os.Args); err != nil {
		if debug {
			logrus.Errorf(err.Error())
		} else {
			fmt.Fprintln(os.Stderr, err.Error())
		}
	} else {
		// The exitCode modified from 125, indicates an application
		// running inside of a container failed, as opposed to the
		// podman command failed.  Must exit with that exit code
		// otherwise command exited correctly.
		if exitCode == 125 {
			exitCode = 0
		}
	}
	os.Exit(exitCode)
}
