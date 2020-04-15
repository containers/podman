package main

import (
	"context"
	"fmt"
	"log/syslog"
	"os"
	"path"
	"runtime/pprof"

	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/tracing"
	"github.com/containers/libpod/version"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	logrusSyslog "github.com/sirupsen/logrus/hooks/syslog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	rootCmd = &cobra.Command{
		Use:                path.Base(os.Args[0]),
		Long:               "Manage pods, containers and images",
		SilenceUsage:       true,
		SilenceErrors:      true,
		TraverseChildren:   true,
		PersistentPreRunE:  preRunE,
		RunE:               registry.SubCommandExists,
		PersistentPostRunE: postRunE,
		Version:            version.Version,
	}

	logLevels = entities.NewStringSet("debug", "info", "warn", "error", "fatal", "panic")
	logLevel  = "error"
	useSyslog bool
)

func init() {
	cobra.OnInitialize(
		rootlessHook,
		loggingHook,
		syslogHook,
	)

	rootFlags(registry.PodmanOptions, rootCmd.PersistentFlags())
}

func Execute() {
	if err := rootCmd.ExecuteContext(registry.GetContextWithOptions()); err != nil {
		logrus.Error(err)
	} else if registry.GetExitCode() == registry.ExecErrorCodeGeneric {
		// The exitCode modified from registry.ExecErrorCodeGeneric,
		// indicates an application
		// running inside of a container failed, as opposed to the
		// podman command failed.  Must exit with that exit code
		// otherwise command exited correctly.
		registry.SetExitCode(0)
	}
	os.Exit(registry.GetExitCode())
}

func preRunE(cmd *cobra.Command, _ []string) error {
	// Update PodmanOptions now that we "know" more
	// TODO: pass in path overriding configuration file
	registry.PodmanOptions = registry.NewPodmanConfig()

	cmd.SetHelpTemplate(registry.HelpTemplate())
	cmd.SetUsageTemplate(registry.UsageTemplate())

	if cmd.Flag("cpu-profile").Changed {
		f, err := os.Create(registry.PodmanOptions.CpuProfile)
		if err != nil {
			return errors.Wrapf(err, "unable to create cpu profiling file %s",
				registry.PodmanOptions.CpuProfile)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			return err
		}
	}

	if cmd.Flag("trace").Changed {
		tracer, closer := tracing.Init("podman")
		opentracing.SetGlobalTracer(tracer)
		registry.PodmanOptions.SpanCloser = closer

		registry.PodmanOptions.Span = tracer.StartSpan("before-context")
		registry.PodmanOptions.SpanCtx = opentracing.ContextWithSpan(context.Background(), registry.PodmanOptions.Span)
	}
	return nil
}

func postRunE(cmd *cobra.Command, args []string) error {
	if cmd.Flag("cpu-profile").Changed {
		pprof.StopCPUProfile()
	}
	if cmd.Flag("trace").Changed {
		registry.PodmanOptions.Span.Finish()
		registry.PodmanOptions.SpanCloser.Close()
	}
	return nil
}

func loggingHook() {
	if !logLevels.Contains(logLevel) {
		logrus.Errorf("Log Level \"%s\" is not supported, choose from: %s", logLevel, logLevels.String())
		os.Exit(1)
	}

	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}
	logrus.SetLevel(level)

	if logrus.IsLevelEnabled(logrus.InfoLevel) {
		logrus.Infof("%s filtering at log level %s", os.Args[0], logrus.GetLevel())
	}
}

func syslogHook() {
	if useSyslog {
		hook, err := logrusSyslog.NewSyslogHook("", "", syslog.LOG_INFO, "")
		if err != nil {
			logrus.WithError(err).Error("Failed to initialize syslog hook")
		}
		if err == nil {
			logrus.AddHook(hook)
		}
	}
}

func rootlessHook() {
	if rootless.IsRootless() {
		logrus.Error("rootless mode is currently not supported. Support will return ASAP.")
	}
	// ce, err := registry.NewContainerEngine(rootCmd, []string{})
	// if err != nil {
	// 	logrus.WithError(err).Fatal("failed to obtain container engine")
	// }
	// ce.SetupRootLess(rootCmd)
}

func rootFlags(opts entities.PodmanConfig, flags *pflag.FlagSet) {
	// V2 flags
	flags.StringVarP(&opts.Uri, "remote", "r", "", "URL to access Podman service")
	flags.StringSliceVar(&opts.Identities, "identity", []string{}, "path to SSH identity file")

	// Override default --help information of `--version` global flag
	// TODO: restore -v option for version without breaking -v for volumes
	var dummyVersion bool
	flags.BoolVar(&dummyVersion, "version", false, "Version of Podman")

	cfg := opts.Config
	flags.StringVar(&cfg.Engine.CgroupManager, "cgroup-manager", cfg.Engine.CgroupManager, opts.CGroupUsage)
	flags.StringVar(&opts.CpuProfile, "cpu-profile", "", "Path for the cpu profiling results")
	flags.StringVar(&opts.ConmonPath, "conmon", "", "Path of the conmon binary")
	flags.StringVar(&cfg.Engine.NetworkCmdPath, "network-cmd-path", cfg.Engine.NetworkCmdPath, "Path to the command for configuring the network")
	flags.StringVar(&cfg.Network.NetworkConfigDir, "cni-config-dir", cfg.Network.NetworkConfigDir, "Path of the configuration directory for CNI networks")
	flags.StringVar(&cfg.Containers.DefaultMountsFile, "default-mounts-file", cfg.Containers.DefaultMountsFile, "Path to default mounts file")
	flags.StringVar(&cfg.Engine.EventsLogger, "events-backend", cfg.Engine.EventsLogger, `Events backend to use ("file"|"journald"|"none")`)
	flags.StringSliceVar(&cfg.Engine.HooksDir, "hooks-dir", cfg.Engine.HooksDir, "Set the OCI hooks directory path (may be set multiple times)")
	flags.IntVar(&opts.MaxWorks, "max-workers", 0, "The maximum number of workers for parallel operations")
	flags.StringVar(&cfg.Engine.Namespace, "namespace", cfg.Engine.Namespace, "Set the libpod namespace, used to create separate views of the containers and pods on the system")
	flags.StringVar(&cfg.Engine.StaticDir, "root", "", "Path to the root directory in which data, including images, is stored")
	flags.StringVar(&opts.Runroot, "runroot", "", "Path to the 'run directory' where all state information is stored")
	flags.StringVar(&opts.RuntimePath, "runtime", "", "Path to the OCI-compatible binary used to run containers, default is /usr/bin/runc")
	// -s is deprecated due to conflict with -s on subcommands
	flags.StringVar(&opts.StorageDriver, "storage-driver", "", "Select which storage driver is used to manage storage of images and containers (default is overlay)")
	flags.StringArrayVar(&opts.StorageOpts, "storage-opt", []string{}, "Used to pass an option to the storage driver")

	flags.StringVar(&opts.Engine.TmpDir, "tmpdir", "", "Path to the tmp directory for libpod state content.\n\nNote: use the environment variable 'TMPDIR' to change the temporary storage location for container images, '/var/tmp'.\n")
	flags.BoolVar(&opts.Trace, "trace", false, "Enable opentracing output (default false)")

	// Override default --help information of `--help` global flag
	var dummyHelp bool
	flags.BoolVar(&dummyHelp, "help", false, "Help for podman")
	flags.StringVar(&logLevel, "log-level", logLevel, fmt.Sprintf("Log messages above specified level (%s)", logLevels.String()))

	// Hide these flags for both ABI and Tunneling
	for _, f := range []string{
		"cpu-profile",
		"default-mounts-file",
		"max-workers",
		"trace",
	} {
		if err := flags.MarkHidden(f); err != nil {
			logrus.Warnf("unable to mark %s flag as hidden", f)
		}
	}

	// Only create these flags for ABI connections
	if !registry.IsRemote() {
		flags.BoolVar(&useSyslog, "syslog", false, "Output logging information to syslog as well as the console (default false)")
	}

}
