package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/validate"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/parallel"
	"github.com/containers/podman/v3/pkg/rootless"
	"github.com/containers/podman/v3/version"
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
		Use:                   filepath.Base(os.Args[0]) + " [options]",
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

	logLevel       = "warn"
	useSyslog      bool
	requireCleanup = true
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
	logrus.Debugf("Called %s.PersistentPreRunE(%s)", cmd.Name(), strings.Join(os.Args, " "))

	// Help, completion and commands with subcommands are special cases, no need for more setup
	// Completion cmd is used to generate the shell scripts
	if cmd.Name() == "help" || cmd.Name() == "completion" || cmd.HasSubCommands() {
		requireCleanup = false
		return nil
	}

	cfg := registry.PodmanConfig()

	// --connection is not as "special" as --remote so we can wait and process it here
	conn := cmd.Root().LocalFlags().Lookup("connection")
	if conn != nil && conn.Changed {
		cfg.Engine.ActiveService = conn.Value.String()

		var err error
		cfg.URI, cfg.Identity, err = cfg.ActiveDestination()
		if err != nil {
			return errors.Wrap(err, "failed to resolve active destination")
		}

		if err := cmd.Root().LocalFlags().Set("url", cfg.URI); err != nil {
			return errors.Wrap(err, "failed to override --url flag")
		}

		if err := cmd.Root().LocalFlags().Set("identity", cfg.Identity); err != nil {
			return errors.Wrap(err, "failed to override --identity flag")
		}
	}

	// Special case if command is hidden completion command ("__complete","__completeNoDesc")
	// Since __completeNoDesc is an alias the cm.Name is always __complete
	if cmd.Name() == cobra.ShellCompRequestCmd {
		// Parse the cli arguments after the the completion cmd (always called as second argument)
		// This ensures that the --url, --identity and --connection flags are properly set
		compCmd, _, err := cmd.Root().Traverse(os.Args[2:])
		if err != nil {
			return err
		}
		// If we don't complete the root cmd hide all root flags
		// so they won't show up in the completions on subcommands.
		if compCmd != compCmd.Root() {
			compCmd.Root().Flags().VisitAll(func(flag *pflag.Flag) {
				flag.Hidden = true
			})
		}
		// No need for further setup the completion logic setups the engines as needed.
		requireCleanup = false
		return nil
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
	// Hard code TMPDIR functions to use /var/tmp, if user did not override
	if _, ok := os.LookupEnv("TMPDIR"); !ok {
		os.Setenv("TMPDIR", "/var/tmp")
	}

	context := cmd.Root().LocalFlags().Lookup("context")
	if context.Value.String() != "default" {
		return errors.New("Podman does not support swarm, the only --context value allowed is \"default\"")
	}
	if !registry.IsRemote() {
		if cmd.Flag("cpu-profile").Changed {
			f, err := os.Create(cfg.CPUProfile)
			if err != nil {
				return err
			}
			if err := pprof.StartCPUProfile(f); err != nil {
				return err
			}
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
	logrus.Debugf("Called %s.PersistentPostRunE(%s)", cmd.Name(), strings.Join(os.Args, " "))

	if !requireCleanup {
		return nil
	}

	if !registry.IsRemote() {
		if cmd.Flag("cpu-profile").Changed {
			pprof.StopCPUProfile()
		}
	}

	registry.ImageEngine().Shutdown(registry.Context())
	registry.ContainerEngine().Shutdown(registry.Context())
	return nil
}

func loggingHook() {
	var found bool
	for _, l := range common.LogLevels {
		if l == strings.ToLower(logLevel) {
			found = true
			break
		}
	}
	if !found {
		fmt.Fprintf(os.Stderr, "Log Level %q is not supported, choose from: %s\n", logLevel, strings.Join(common.LogLevels, ", "))
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

	connectionFlagName := "connection"
	lFlags.StringVarP(&opts.Engine.ActiveService, connectionFlagName, "c", srv, "Connection to use for remote Podman service")
	_ = cmd.RegisterFlagCompletionFunc(connectionFlagName, common.AutocompleteSystemConnections)

	urlFlagName := "url"
	lFlags.StringVar(&opts.URI, urlFlagName, uri, "URL to access Podman service (CONTAINER_HOST)")
	_ = cmd.RegisterFlagCompletionFunc(urlFlagName, completion.AutocompleteDefault)

	// Context option added just for compatibility with DockerCLI.
	lFlags.String("context", "default", "Name of the context to use to connect to the daemon (This flag is a NOOP and provided solely for scripting compatibility.)")
	_ = lFlags.MarkHidden("context")

	identityFlagName := "identity"
	lFlags.StringVar(&opts.Identity, identityFlagName, ident, "path to SSH identity file, (CONTAINER_SSHKEY)")
	_ = cmd.RegisterFlagCompletionFunc(identityFlagName, completion.AutocompleteDefault)

	lFlags.BoolVarP(&opts.Remote, "remote", "r", false, "Access remote Podman service (default false)")
	pFlags := cmd.PersistentFlags()
	if registry.IsRemote() {
		if err := lFlags.MarkHidden("remote"); err != nil {
			logrus.Warnf("unable to mark --remote flag as hidden: %s", err.Error())
		}
		opts.Remote = true
	} else {
		cgroupManagerFlagName := "cgroup-manager"
		pFlags.StringVar(&cfg.Engine.CgroupManager, cgroupManagerFlagName, cfg.Engine.CgroupManager, "Cgroup manager to use (\"cgroupfs\"|\"systemd\")")
		_ = cmd.RegisterFlagCompletionFunc(cgroupManagerFlagName, common.AutocompleteCgroupManager)

		pFlags.StringVar(&opts.CPUProfile, "cpu-profile", "", "Path for the cpu profiling results")

		conmonFlagName := "conmon"
		pFlags.StringVar(&opts.ConmonPath, conmonFlagName, "", "Path of the conmon binary")
		_ = cmd.RegisterFlagCompletionFunc(conmonFlagName, completion.AutocompleteDefault)

		networkCmdPathFlagName := "network-cmd-path"
		pFlags.StringVar(&cfg.Engine.NetworkCmdPath, networkCmdPathFlagName, cfg.Engine.NetworkCmdPath, "Path to the command for configuring the network")
		_ = cmd.RegisterFlagCompletionFunc(networkCmdPathFlagName, completion.AutocompleteDefault)

		cniConfigDirFlagName := "cni-config-dir"
		pFlags.StringVar(&cfg.Network.NetworkConfigDir, cniConfigDirFlagName, cfg.Network.NetworkConfigDir, "Path of the configuration directory for CNI networks")
		_ = cmd.RegisterFlagCompletionFunc(cniConfigDirFlagName, completion.AutocompleteDefault)

		pFlags.StringVar(&cfg.Containers.DefaultMountsFile, "default-mounts-file", cfg.Containers.DefaultMountsFile, "Path to default mounts file")

		eventsBackendFlagName := "events-backend"
		pFlags.StringVar(&cfg.Engine.EventsLogger, eventsBackendFlagName, cfg.Engine.EventsLogger, `Events backend to use ("file"|"journald"|"none")`)
		_ = cmd.RegisterFlagCompletionFunc(eventsBackendFlagName, common.AutocompleteEventBackend)

		hooksDirFlagName := "hooks-dir"
		pFlags.StringSliceVar(&cfg.Engine.HooksDir, hooksDirFlagName, cfg.Engine.HooksDir, "Set the OCI hooks directory path (may be set multiple times)")
		_ = cmd.RegisterFlagCompletionFunc(hooksDirFlagName, completion.AutocompleteDefault)

		pFlags.IntVar(&opts.MaxWorks, "max-workers", (runtime.NumCPU()*3)+1, "The maximum number of workers for parallel operations")

		namespaceFlagName := "namespace"
		pFlags.StringVar(&cfg.Engine.Namespace, namespaceFlagName, cfg.Engine.Namespace, "Set the libpod namespace, used to create separate views of the containers and pods on the system")
		_ = cmd.RegisterFlagCompletionFunc(namespaceFlagName, completion.AutocompleteNone)

		rootFlagName := "root"
		pFlags.StringVar(&cfg.Engine.StaticDir, rootFlagName, "", "Path to the root directory in which data, including images, is stored")
		_ = cmd.RegisterFlagCompletionFunc(rootFlagName, completion.AutocompleteDefault)

		pFlags.StringVar(&opts.RegistriesConf, "registries-conf", "", "Path to a registries.conf to use for image processing")

		runrootFlagName := "runroot"
		pFlags.StringVar(&opts.Runroot, runrootFlagName, "", "Path to the 'run directory' where all state information is stored")
		_ = cmd.RegisterFlagCompletionFunc(runrootFlagName, completion.AutocompleteDefault)

		runtimeFlagName := "runtime"
		pFlags.StringVar(&opts.RuntimePath, runtimeFlagName, "", "Path to the OCI-compatible binary used to run containers, default is /usr/bin/runc")
		_ = cmd.RegisterFlagCompletionFunc(runtimeFlagName, completion.AutocompleteDefault)

		// -s is deprecated due to conflict with -s on subcommands
		storageDriverFlagName := "storage-driver"
		pFlags.StringVar(&opts.StorageDriver, storageDriverFlagName, "", "Select which storage driver is used to manage storage of images and containers (default is overlay)")
		_ = cmd.RegisterFlagCompletionFunc(storageDriverFlagName, completion.AutocompleteNone) //TODO: what can we recommend here?

		storageOptFlagName := "storage-opt"
		pFlags.StringArrayVar(&opts.StorageOpts, storageOptFlagName, []string{}, "Used to pass an option to the storage driver")
		_ = cmd.RegisterFlagCompletionFunc(storageOptFlagName, completion.AutocompleteNone)

		tmpdirFlagName := "tmpdir"
		pFlags.StringVar(&opts.Engine.TmpDir, tmpdirFlagName, "", "Path to the tmp directory for libpod state content.\n\nNote: use the environment variable 'TMPDIR' to change the temporary storage location for container images, '/var/tmp'.\n")
		_ = cmd.RegisterFlagCompletionFunc(tmpdirFlagName, completion.AutocompleteDefault)

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

	logLevelFlagName := "log-level"
	pFlags.StringVar(&logLevel, logLevelFlagName, logLevel, fmt.Sprintf("Log messages above specified level (%s)", strings.Join(common.LogLevels, ", ")))
	_ = rootCmd.RegisterFlagCompletionFunc(logLevelFlagName, common.AutocompleteLogLevel)

	// Only create these flags for ABI connections
	if !registry.IsRemote() {
		runtimeflagFlagName := "runtime-flag"
		pFlags.StringArrayVar(&opts.RuntimeFlags, runtimeflagFlagName, []string{}, "add global flags for the container runtime")
		_ = rootCmd.RegisterFlagCompletionFunc(runtimeflagFlagName, completion.AutocompleteNone)

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
		if logrus.IsLevelEnabled(logrus.TraceLevel) {
			message = fmt.Sprintf("Error: %+v", err)
		} else {
			message = fmt.Sprintf("Error: %v", err)
		}
	}
	return message
}
