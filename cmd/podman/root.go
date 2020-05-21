package main

import (
	"fmt"
	"os"
	"path"
	"runtime/pprof"
	"strings"

	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/cmd/podman/validate"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/tracing"
	"github.com/containers/libpod/version"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// HelpTemplate is the help template for podman commands
// This uses the short and long options.
// command should not use this.
const helpTemplate = `{{.Short}}

Description:
  {{.Long}}

{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`

// UsageTemplate is the usage template for podman commands
// This blocks the displaying of the global options. The main podman
// command should not use this.
const usageTemplate = `Usage:{{if (and .Runnable (not .HasAvailableSubCommands))}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
  {{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}
{{end}}
`

var (
	rootCmd = &cobra.Command{
		Use:                path.Base(os.Args[0]),
		Long:               "Manage pods, containers and images",
		SilenceUsage:       true,
		SilenceErrors:      true,
		TraverseChildren:   true,
		PersistentPreRunE:  persistentPreRunE,
		RunE:               validate.SubCommandExists,
		PersistentPostRunE: persistentPostRunE,
		Version:            version.Version,
	}

	logLevels = []string{"debug", "info", "warn", "error", "fatal", "panic"}
	logLevel  = "error"
	useSyslog bool
)

func init() {
	// Hooks are called before PersistentPreRunE()
	cobra.OnInitialize(
		loggingHook,
		syslogHook,
	)

	rootFlags(registry.PodmanConfig(), rootCmd.PersistentFlags())

	// "version" is a local flag to avoid collisions with sub-commands that use "-v"
	var dummyVersion bool
	rootCmd.Flags().BoolVarP(&dummyVersion, "version", "v", false, "Version of Podman")
}

func Execute() {
	if err := rootCmd.ExecuteContext(registry.GetContextWithOptions()); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err.Error())
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

func persistentPreRunE(cmd *cobra.Command, args []string) error {
	// TODO: Remove trace statement in podman V2.1
	logrus.Debugf("Called %s.PersistentPreRunE(%s)", cmd.Name(), strings.Join(os.Args, " "))

	cfg := registry.PodmanConfig()

	// Prep the engines
	if _, err := registry.NewImageEngine(cmd, args); err != nil {
		return err
	}
	if _, err := registry.NewContainerEngine(cmd, args); err != nil {
		return err
	}

	if cmd.Flag("cpu-profile").Changed {
		f, err := os.Create(cfg.CpuProfile)
		if err != nil {
			return errors.Wrapf(err, "unable to create cpu profiling file %s",
				cfg.CpuProfile)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			return err
		}
	}

	if cmd.Flag("trace").Changed {
		tracer, closer := tracing.Init("podman")
		opentracing.SetGlobalTracer(tracer)
		cfg.SpanCloser = closer

		cfg.Span = tracer.StartSpan("before-context")
		cfg.SpanCtx = opentracing.ContextWithSpan(registry.Context(), cfg.Span)
		opentracing.StartSpanFromContext(cfg.SpanCtx, cmd.Name())
	}

	// Setup Rootless environment, IFF:
	// 1) in ABI mode
	// 2) running as non-root
	// 3) command doesn't require Parent Namespace
	_, found := cmd.Annotations[registry.ParentNSRequired]
	if !registry.IsRemote() && rootless.IsRootless() && !found {
		err := registry.ContainerEngine().SetupRootless(registry.Context(), cmd)
		if err != nil {
			return err
		}
	}
	return nil
}

func persistentPostRunE(cmd *cobra.Command, args []string) error {
	// TODO: Remove trace statement in podman V2.1
	logrus.Debugf("Called %s.PersistentPostRunE(%s)", cmd.Name(), strings.Join(os.Args, " "))

	cfg := registry.PodmanConfig()
	if cmd.Flag("cpu-profile").Changed {
		pprof.StopCPUProfile()
	}
	if cmd.Flag("trace").Changed {
		cfg.Span.Finish()
		cfg.SpanCloser.Close()
	}

	registry.ImageEngine().Shutdown(registry.Context())
	registry.ContainerEngine().Shutdown(registry.Context())
	return nil
}

func loggingHook() {
	var found bool
	for _, l := range logLevels {
		if l == logLevel {
			found = true
			break
		}
	}
	if !found {
		fmt.Fprintf(os.Stderr, "Log Level \"%s\" is not supported, choose from: %s\n", logLevel, strings.Join(logLevels, ", "))
		os.Exit(1)
	}

	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
	logrus.SetLevel(level)

	if logrus.IsLevelEnabled(logrus.InfoLevel) {
		logrus.Infof("%s filtering at log level %s", os.Args[0], logrus.GetLevel())
	}
}

func rootFlags(opts *entities.PodmanConfig, flags *pflag.FlagSet) {
	// V2 flags
	flags.StringVarP(&opts.Uri, "remote", "r", registry.DefaultAPIAddress(), "URL to access Podman service")
	flags.StringSliceVar(&opts.Identities, "identity", []string{}, "path to SSH identity file")

	cfg := opts.Config
	flags.StringVar(&cfg.Engine.CgroupManager, "cgroup-manager", cfg.Engine.CgroupManager, "Cgroup manager to use (\"cgroupfs\"|\"systemd\")")
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
	flags.StringVar(&logLevel, "log-level", logLevel, fmt.Sprintf("Log messages above specified level (%s)", strings.Join(logLevels, ", ")))

	// Hide these flags for both ABI and Tunneling
	for _, f := range []string{
		"cpu-profile",
		"default-mounts-file",
		"max-workers",
		"trace",
	} {
		if err := flags.MarkHidden(f); err != nil {
			logrus.Warnf("unable to mark %s flag as hidden: %s", f, err.Error())
		}
	}

	// Only create these flags for ABI connections
	if !registry.IsRemote() {
		flags.BoolVar(&useSyslog, "syslog", false, "Output logging information to syslog as well as the console (default false)")
	}
}
