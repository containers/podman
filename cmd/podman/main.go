package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime/pprof"
	"syscall"

	"github.com/containers/libpod/libpod"
	_ "github.com/containers/libpod/pkg/hooks/0.1.0"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/version"
	"github.com/containers/storage/pkg/reexec"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	lsyslog "github.com/sirupsen/logrus/hooks/syslog"
	"github.com/urfave/cli"
	"log/syslog"
)

// This is populated by the Makefile from the VERSION file
// in the repository
var (
	exitCode = 125
)

var cmdsNotRequiringRootless = map[string]bool{
	"help":    true,
	"version": true,
	"create":  true,
	"exec":    true,
	// `info` must be executed in an user namespace.
	// If this change, please also update libpod.refreshRootless()
	"login":   true,
	"logout":  true,
	"kill":    true,
	"pause":   true,
	"run":     true,
	"unpause": true,
	"search":  true,
	"stats":   true,
	"stop":    true,
	"top":     true,
}

func main() {
	debug := false
	cpuProfile := false

	if reexec.Init() {
		return
	}

	app := cli.NewApp()
	app.Name = "podman"
	app.Usage = "manage pods and images"
	app.OnUsageError = usageErrorHandler
	app.CommandNotFound = commandNotFoundHandler

	app.Version = version.Version

	app.Commands = []cli.Command{
		attachCommand,
		commitCommand,
		containerCommand,
		buildCommand,
		createCommand,
		diffCommand,
		execCommand,
		exportCommand,
		historyCommand,
		imageCommand,
		imagesCommand,
		importCommand,
		infoCommand,
		inspectCommand,
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
		pullCommand,
		pushCommand,
		restartCommand,
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

	if varlinkCommand != nil {
		app.Commands = append(app.Commands, *varlinkCommand)
	}

	app.Before = func(c *cli.Context) error {
		if err := libpod.SetXdgRuntimeDir(""); err != nil {
			logrus.Errorf(err.Error())
			os.Exit(1)
		}
		args := c.Args()
		if args.Present() {
			if _, notRequireRootless := cmdsNotRequiringRootless[args.First()]; !notRequireRootless {
				became, ret, err := rootless.BecomeRootInUserNS()
				if err != nil {
					logrus.Errorf(err.Error())
					os.Exit(1)
				}
				if became {
					os.Exit(ret)
				}
			}
		}
		if c.GlobalBool("syslog") {
			hook, err := lsyslog.NewSyslogHook("", "", syslog.LOG_INFO, "")
			if err == nil {
				logrus.AddHook(hook)
			}
		}
		logLevel := c.GlobalString("log-level")
		if logLevel != "" {
			level, err := logrus.ParseLevel(logLevel)
			if err != nil {
				return err
			}

			logrus.SetLevel(level)
		}

		// Only if not rootless, set rlimits for open files.
		// We open numerous FDs for ports opened
		if !rootless.IsRootless() {
			rlimits := new(syscall.Rlimit)
			rlimits.Cur = 1048576
			rlimits.Max = 1048576
			if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, rlimits); err != nil {
				return errors.Wrapf(err, "error setting new rlimits")
			}
		} else {
			logrus.Info("running as rootless")
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
			Name:  "cgroup-manager",
			Usage: "cgroup manager to use (cgroupfs or systemd, default systemd)",
		},
		cli.StringFlag{
			Name:  "cni-config-dir",
			Usage: "path of the configuration directory for CNI networks",
		},
		cli.StringFlag{
			Name:   "config, c",
			Usage:  "path of a libpod config file detailing container server configuration options",
			Hidden: true,
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
			Name:   "default-mounts-file",
			Usage:  "path to default mounts file",
			Hidden: true,
		},
		cli.StringSliceFlag{
			Name:  "hooks-dir",
			Usage: "set the OCI hooks directory path (may be set multiple times)",
		},
		cli.IntFlag{
			Name:   "max-workers",
			Usage:  "the maximum number of workers for parallel operations",
			Hidden: true,
		},
		cli.StringFlag{
			Name:  "log-level",
			Usage: "log messages above specified level: debug, info, warn, error (default), fatal or panic",
			Value: "error",
		},
		cli.StringFlag{
			Name:  "namespace",
			Usage: "set the libpod namespace, used to create separate views of the containers and pods on the system",
			Value: "",
		},
		cli.StringFlag{
			Name:  "root",
			Usage: "path to the root directory in which data, including images, is stored",
		},
		cli.StringFlag{
			Name:  "tmpdir",
			Usage: "path to the tmp directory",
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
		cli.BoolFlag{
			Name:  "syslog",
			Usage: "output logging information to syslog as well as the console",
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
			// Retrieve the exit error from the exec call, if it exists
			if ee, ok := err.(*exec.ExitError); ok {
				if status, ok := ee.Sys().(syscall.WaitStatus); ok {
					exitCode = status.ExitStatus()
				}
			}
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
