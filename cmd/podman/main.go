package main

import (
	"fmt"
	"log/syslog"
	"os"
	"os/exec"
	"runtime/pprof"
	"sort"
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
	"export":  true,
	// `info` must be executed in an user namespace.
	// If this change, please also update libpod.refreshRootless()
	"login":   true,
	"logout":  true,
	"mount":   true,
	"kill":    true,
	"pause":   true,
	"restart": true,
	"run":     true,
	"unpause": true,
	"search":  true,
	"stats":   true,
	"stop":    true,
	"top":     true,
}

type commandSorted []cli.Command

func (a commandSorted) Len() int      { return len(a) }
func (a commandSorted) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

type commandSortedAlpha struct{ commandSorted }

func (a commandSortedAlpha) Less(i, j int) bool {
	return a.commandSorted[i].Name < a.commandSorted[j].Name
}

type flagSorted []cli.Flag

func (a flagSorted) Len() int      { return len(a) }
func (a flagSorted) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

type flagSortedAlpha struct{ flagSorted }

func (a flagSortedAlpha) Less(i, j int) bool {
	return a.flagSorted[i].GetName() < a.flagSorted[j].GetName()
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
		containerCommand,
		exportCommand,
		historyCommand,
		imageCommand,
		imagesCommand,
		importCommand,
		infoCommand,
		inspectCommand,
		pullCommand,
		rmiCommand,
		systemCommand,
		tagCommand,
		versionCommand,
	}

	app.Commands = append(app.Commands, getAppCommands()...)
	sort.Sort(commandSortedAlpha{app.Commands})

	if varlinkCommand != nil {
		app.Commands = append(app.Commands, *varlinkCommand)
	}

	app.Before = func(c *cli.Context) error {
		if err := libpod.SetXdgRuntimeDir(""); err != nil {
			logrus.Errorf(err.Error())
			os.Exit(1)
		}
		args := c.Args()
		if args.Present() && rootless.IsRootless() {
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

		rlimits := new(syscall.Rlimit)
		rlimits.Cur = 1048576
		rlimits.Max = 1048576
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, rlimits); err != nil {
			if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, rlimits); err != nil {
				return errors.Wrapf(err, "error getting rlimits")
			}
			rlimits.Cur = rlimits.Max
			if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, rlimits); err != nil {
				return errors.Wrapf(err, "error setting new rlimits")
			}
		}

		if rootless.IsRootless() {
			logrus.Info("running as rootless")
		}

		// Be sure we can create directories with 0755 mode.
		syscall.Umask(0022)

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
			Name:   "config, c",
			Usage:  "path of a libpod config file detailing container server configuration options",
			Hidden: true,
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
			Name:  "tmpdir",
			Usage: "path to the tmp directory",
		},
	}

	app.Flags = append(app.Flags, getMainAppFlags()...)
	sort.Sort(flagSortedAlpha{app.Flags})

	// Check if /etc/containers/registries.conf exists when running in
	// in a local environment.
	CheckForRegistries()

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
