package main

import (
	"fmt"
	"log/syslog"
	"os"
	"os/exec"
	"runtime/pprof"
	"strings"
	"syscall"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod"
	_ "github.com/containers/libpod/pkg/hooks/0.1.0"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/version"
	"github.com/containers/storage/pkg/reexec"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	lsyslog "github.com/sirupsen/logrus/hooks/syslog"
	"github.com/spf13/cobra"
)

// This is populated by the Makefile from the VERSION file
// in the repository
var (
	exitCode = 125
)

// Commands that the remote and local client have
// implemented.
var mainCommands = []*cobra.Command{
	_exportCommand,
	_historyCommand,
	_imagesCommand,
	_importCommand,
	_infoCommand,
	_inspectCommand,
	_killCommand,
	_pullCommand,
	_pushCommand,
	_rmiCommand,
	_tagCommand,
	_versionCommand,
	imageCommand.Command,
	systemCommand.Command,
}

var cmdsNotRequiringRootless = map[*cobra.Command]bool{
	_versionCommand: true,
	_createCommand:  true,
	_execCommand:    true,
	_exportCommand:  true,
	//// `info` must be executed in an user namespace.
	//// If this change, please also update libpod.refreshRootless()
	_loginCommand:   true,
	_logoutCommand:  true,
	_mountCommand:   true,
	_killCommand:    true,
	_pauseCommand:   true,
	_restartCommand: true,
	_runCommand:     true,
	_unpauseCommand: true,
	_searchCommand:  true,
	_statsCommand:   true,
	_stopCommand:    true,
	_topCommand:     true,
}

var rootCmd = &cobra.Command{
	Use:  "podman",
	Long: "manage pods and images",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return before(cmd, args)
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return after(cmd, args)
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

var MainGlobalOpts cliconfig.MainFlags

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.TraverseChildren = true
	rootCmd.Version = version.Version
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.CGroupManager, "cgroup-manager", "", "Cgroup manager to use (cgroupfs or systemd, default systemd)")
	// -c is deprecated due to conflict with -c on subcommands
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.CpuProfile, "cpu-profile", "", "Path for the cpu profiling results")
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.Config, "config", "", "Path of a libpod config file detailing container server configuration options")
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.ConmonPath, "conmon", "", "Path of the conmon binary")
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.CniConfigDir, "cni-config-dir", "", "Path of the configuration directory for CNI networks")
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.DefaultMountsFile, "default-mounts-file", "", "Path to default mounts file")
	rootCmd.PersistentFlags().MarkHidden("defaults-mount-file")
	rootCmd.PersistentFlags().StringSliceVar(&MainGlobalOpts.HooksDir, "hooks-dir", []string{}, "Set the OCI hooks directory path (may be set multiple times)")
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.LogLevel, "log-level", "error", "Log messages above specified level: debug, info, warn, error (default), fatal or panic")
	rootCmd.PersistentFlags().IntVar(&MainGlobalOpts.MaxWorks, "max-workers", 0, "The maximum number of workers for parallel operations")
	rootCmd.PersistentFlags().MarkHidden("max-workers")
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.Namespace, "namespace", "", "Set the libpod namespace, used to create separate views of the containers and pods on the system")
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.Root, "root", "", "Path to the root directory in which data, including images, is stored")
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.Runroot, "runroot", "", "Path to the 'run directory' where all state information is stored")
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.Runtime, "runtime", "", "Path to the OCI-compatible binary used to run containers, default is /usr/bin/runc")
	// -s is depracated due to conflict with -s on subcommands
	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.StorageDriver, "storage-driver", "", "Select which storage driver is used to manage storage of images and containers (default is overlay)")
	rootCmd.PersistentFlags().StringSliceVar(&MainGlobalOpts.StorageOpts, "storage-opt", []string{}, "Used to pass an option to the storage driver")
	rootCmd.PersistentFlags().BoolVar(&MainGlobalOpts.Syslog, "syslog", false, "Output logging information to syslog as well as the console")

	rootCmd.PersistentFlags().StringVar(&MainGlobalOpts.TmpDir, "tmpdir", "", "Path to the tmp directory")
	rootCmd.AddCommand(mainCommands...)
	rootCmd.AddCommand(getMainCommands()...)

}
func initConfig() {
	//	we can do more stuff in here.
}

func before(cmd *cobra.Command, args []string) error {
	if err := libpod.SetXdgRuntimeDir(""); err != nil {
		logrus.Errorf(err.Error())
		os.Exit(1)
	}
	if rootless.IsRootless() {
		notRequireRootless := cmdsNotRequiringRootless[cmd]
		if !notRequireRootless && !strings.HasPrefix(cmd.Use, "help") {
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

	if MainGlobalOpts.Syslog {
		hook, err := lsyslog.NewSyslogHook("", "", syslog.LOG_INFO, "")
		if err == nil {
			logrus.AddHook(hook)
		}
	}

	//	Set log level
	level, err := logrus.ParseLevel(MainGlobalOpts.LogLevel)
	if err != nil {
		return err
	}
	logrus.SetLevel(level)

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

	if cmd.Flag("cpu-profile").Changed {
		f, err := os.Create(MainGlobalOpts.CpuProfile)
		if err != nil {
			return errors.Wrapf(err, "unable to create cpu profiling file %s",
				MainGlobalOpts.CpuProfile)
		}
		pprof.StartCPUProfile(f)
	}
	return nil
}

func after(cmd *cobra.Command, args []string) error {
	if cmd.Flag("cpu-profile").Changed {
		pprof.StopCPUProfile()
	}
	return nil
}

func main() {
	//debug := false
	//cpuProfile := false

	if reexec.Init() {
		return
	}
	if err := rootCmd.Execute(); err != nil {
		if MainGlobalOpts.LogLevel == "debug" {
			logrus.Errorf(err.Error())
		} else {
			if ee, ok := err.(*exec.ExitError); ok {
				if status, ok := ee.Sys().(syscall.WaitStatus); ok {
					exitCode = status.ExitStatus()
				}
			}
			fmt.Fprintln(os.Stderr, "Error:", err.Error())
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

	// Check if /etc/containers/registries.conf exists when running in
	// in a local environment.
	CheckForRegistries()
	os.Exit(exitCode)
}
