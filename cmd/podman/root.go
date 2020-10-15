package main

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"runtime/pprof"
	"strings"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/validate"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/parallel"
	"github.com/containers/podman/v2/pkg/rootless"
	"github.com/containers/podman/v2/pkg/tracing"
	"github.com/containers/podman/v2/version"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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
  {{.UseLine}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
  {{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Options:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}
{{end}}
`

var (
	rootCmd = &cobra.Command{
		Use:                   path.Base(os.Args[0]) + " [options]",
		Long:                  "Manage pods, containers and images",
		SilenceUsage:          true,
		SilenceErrors:         true,
		TraverseChildren:      true,
		PersistentPreRunE:     persistentPreRunE,
		RunE:                  validate.SubCommandExists,
		PersistentPostRunE:    persistentPostRunE,
		Version:               version.Version.String(),
		DisableFlagsInUseLine: true,
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
		earlyInitHook,
	)

	rootFlags(rootCmd, registry.PodmanConfig())
	rootCmd.SetUsageTemplate(usageTemplate)
}

func Execute() {
	if err := rootCmd.ExecuteContext(registry.GetContextWithOptions()); err != nil {
		fmt.Fprintln(os.Stderr, formatError(err))
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

	// Help and commands with subcommands are special cases, no need for more setup
	if cmd.Name() == "help" || cmd.HasSubCommands() {
		return nil
	}

	cfg := registry.PodmanConfig()

	// --connection is not as "special" as --remote so we can wait and process it here
	var connErr error
	conn := cmd.Root().LocalFlags().Lookup("connection")
	if conn != nil && conn.Changed {
		cfg.Engine.ActiveService = conn.Value.String()

		var err error
		cfg.URI, cfg.Identity, err = cfg.ActiveDestination()
		if err != nil {
			connErr = errors.Wrap(err, "failed to resolve active destination")
		}

		if err := cmd.Root().LocalFlags().Set("url", cfg.URI); err != nil {
			connErr = errors.Wrap(err, "failed to override --url flag")
		}

		if err := cmd.Root().LocalFlags().Set("identity", cfg.Identity); err != nil {
			connErr = errors.Wrap(err, "failed to override --identity flag")
		}
	}
	if connErr != nil {
		return connErr
	}

	// Prep the engines
	if _, err := registry.NewImageEngine(cmd, args); err != nil {
		return err
	}
	if _, err := registry.NewContainerEngine(cmd, args); err != nil {
		return err
	}

	for _, env := range cfg.Engine.Env {
		splitEnv := strings.SplitN(env, "=", 2)
		if len(splitEnv) != 2 {
			return fmt.Errorf("invalid environment variable for engine %s, valid configuration is KEY=value pair", env)
		}
		// skip if the env is already defined
		if _, ok := os.LookupEnv(splitEnv[0]); ok {
			logrus.Debugf("environment variable %s is already defined, skip the settings from containers.conf", splitEnv[0])
			continue
		}
		if err := os.Setenv(splitEnv[0], splitEnv[1]); err != nil {
			return err
		}
	}

	if !registry.IsRemote() {
		if cmd.Flag("cpu-profile").Changed {
			f, err := os.Create(cfg.CPUProfile)
			if err != nil {
				return errors.Wrapf(err, "unable to create cpu profiling file %s",
					cfg.CPUProfile)
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

		if cfg.MaxWorks <= 0 {
			return errors.Errorf("maximum workers must be set to a positive number (got %d)", cfg.MaxWorks)
		}
		if err := parallel.SetMaxThreads(uint(cfg.MaxWorks)); err != nil {
			return err
		}
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

	// Help and commands with subcommands are special cases, no need for more cleanup
	if cmd.Name() == "help" || cmd.HasSubCommands() {
		return nil
	}

	cfg := registry.PodmanConfig()
	if !registry.IsRemote() {
		if cmd.Flag("cpu-profile").Changed {
			pprof.StopCPUProfile()
		}
		if cmd.Flag("trace").Changed {
			cfg.Span.Finish()
			cfg.SpanCloser.Close()
		}
	}

	registry.ImageEngine().Shutdown(registry.Context())
	registry.ContainerEngine().Shutdown(registry.Context())
	return nil
}

func loggingHook() {
	var found bool
	for _, l := range logLevels {
		if l == strings.ToLower(logLevel) {
			found = true
			break
		}
	}
	if !found {
		fmt.Fprintf(os.Stderr, "Log Level %q is not supported, choose from: %s\n", logLevel, strings.Join(logLevels, ", "))
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

func rootFlags(cmd *cobra.Command, opts *entities.PodmanConfig) {
	cfg := opts.Config
	srv, uri, ident := resolveDestination()

	lFlags := cmd.Flags()
	lFlags.StringVarP(&opts.Engine.ActiveService, "connection", "c", srv, "Connection to use for remote Podman service")
	lFlags.StringVar(&opts.URI, "url", uri, "URL to access Podman service (CONTAINER_HOST)")
	lFlags.StringVar(&opts.Identity, "identity", ident, "path to SSH identity file, (CONTAINER_SSHKEY)")

	lFlags.BoolVarP(&opts.Remote, "remote", "r", false, "Access remote Podman service (default false)")
	pFlags := cmd.PersistentFlags()
	if registry.IsRemote() {
		if err := lFlags.MarkHidden("remote"); err != nil {
			logrus.Warnf("unable to mark --remote flag as hidden: %s", err.Error())
		}
		opts.Remote = true
	} else {
		pFlags.StringVar(&cfg.Engine.CgroupManager, "cgroup-manager", cfg.Engine.CgroupManager, "Cgroup manager to use (\"cgroupfs\"|\"systemd\")")
		pFlags.StringVar(&opts.CPUProfile, "cpu-profile", "", "Path for the cpu profiling results")
		pFlags.StringVar(&opts.ConmonPath, "conmon", "", "Path of the conmon binary")
		pFlags.StringVar(&cfg.Engine.NetworkCmdPath, "network-cmd-path", cfg.Engine.NetworkCmdPath, "Path to the command for configuring the network")
		pFlags.StringVar(&cfg.Network.NetworkConfigDir, "cni-config-dir", cfg.Network.NetworkConfigDir, "Path of the configuration directory for CNI networks")
		pFlags.StringVar(&cfg.Containers.DefaultMountsFile, "default-mounts-file", cfg.Containers.DefaultMountsFile, "Path to default mounts file")
		pFlags.StringVar(&cfg.Engine.EventsLogger, "events-backend", cfg.Engine.EventsLogger, `Events backend to use ("file"|"journald"|"none")`)
		pFlags.StringSliceVar(&cfg.Engine.HooksDir, "hooks-dir", cfg.Engine.HooksDir, "Set the OCI hooks directory path (may be set multiple times)")
		pFlags.IntVar(&opts.MaxWorks, "max-workers", (runtime.NumCPU()*3)+1, "The maximum number of workers for parallel operations")
		pFlags.StringVar(&cfg.Engine.Namespace, "namespace", cfg.Engine.Namespace, "Set the libpod namespace, used to create separate views of the containers and pods on the system")
		pFlags.StringVar(&cfg.Engine.StaticDir, "root", "", "Path to the root directory in which data, including images, is stored")
		pFlags.StringVar(&opts.RegistriesConf, "registries-conf", "", "Path to a registries.conf to use for image processing")
		pFlags.StringVar(&opts.Runroot, "runroot", "", "Path to the 'run directory' where all state information is stored")
		pFlags.StringVar(&opts.RuntimePath, "runtime", "", "Path to the OCI-compatible binary used to run containers, default is /usr/bin/runc")
		// -s is deprecated due to conflict with -s on subcommands
		pFlags.StringVar(&opts.StorageDriver, "storage-driver", "", "Select which storage driver is used to manage storage of images and containers (default is overlay)")
		pFlags.StringArrayVar(&opts.StorageOpts, "storage-opt", []string{}, "Used to pass an option to the storage driver")

		pFlags.StringVar(&opts.Engine.TmpDir, "tmpdir", "", "Path to the tmp directory for libpod state content.\n\nNote: use the environment variable 'TMPDIR' to change the temporary storage location for container images, '/var/tmp'.\n")
		pFlags.BoolVar(&opts.Trace, "trace", false, "Enable opentracing output (default false)")

		// Hide these flags for both ABI and Tunneling
		for _, f := range []string{
			"cpu-profile",
			"default-mounts-file",
			"max-workers",
			"registries-conf",
			"trace",
		} {
			if err := pFlags.MarkHidden(f); err != nil {
				logrus.Warnf("unable to mark %s flag as hidden: %s", f, err.Error())
			}
		}
	}
	// Override default --help information of `--help` global flag
	var dummyHelp bool
	pFlags.BoolVar(&dummyHelp, "help", false, "Help for podman")
	pFlags.StringVar(&logLevel, "log-level", logLevel, fmt.Sprintf("Log messages above specified level (%s)", strings.Join(logLevels, ", ")))

	// Only create these flags for ABI connections
	if !registry.IsRemote() {
		pFlags.StringArrayVar(&opts.RuntimeFlags, "runtime-flag", []string{}, "add global flags for the container runtime")
		pFlags.BoolVar(&useSyslog, "syslog", false, "Output logging information to syslog as well as the console (default false)")
	}
}

func resolveDestination() (string, string, string) {
	if uri, found := os.LookupEnv("CONTAINER_HOST"); found {
		var ident string
		if v, found := os.LookupEnv("CONTAINER_SSHKEY"); found {
			ident = v
		}
		return "", uri, ident
	}

	cfg, err := config.ReadCustomConfig()
	if err != nil {
		logrus.Warning(errors.Wrap(err, "unable to read local containers.conf"))
		return "", registry.DefaultAPIAddress(), ""
	}

	uri, ident, err := cfg.ActiveDestination()
	if err != nil {
		return "", registry.DefaultAPIAddress(), ""
	}
	return cfg.Engine.ActiveService, uri, ident
}

func formatError(err error) string {
	var message string
	if errors.Cause(err) == define.ErrOCIRuntime {
		// OCIRuntimeErrors include the reason for the failure in the
		// second to last message in the error chain.
		message = fmt.Sprintf(
			"Error: %s: %s",
			define.ErrOCIRuntime.Error(),
			strings.TrimSuffix(err.Error(), ": "+define.ErrOCIRuntime.Error()),
		)
	} else {
		message = "Error: " + err.Error()
	}
	return message
}
