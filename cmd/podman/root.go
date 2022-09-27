package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/ssh"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/validate"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/checkpoint/crutils"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/parallel"
	"github.com/containers/podman/v4/version"
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

	defaultLogLevel = "warn"
	logLevel        = defaultLogLevel
	dockerConfig    = ""
	debug           bool

	useSyslog      bool
	requireCleanup = true
)

func init() {
	// Hooks are called before PersistentPreRunE()
	cobra.OnInitialize(
		loggingHook,
		syslogHook,
		earlyInitHook,
		configHook,
	)

	rootFlags(rootCmd, registry.PodmanConfig())

	// backwards compat still allow --cni-config-dir
	rootCmd.Flags().SetNormalizeFunc(func(f *pflag.FlagSet, name string) pflag.NormalizedName {
		if name == "cni-config-dir" {
			name = "network-config-dir"
		}
		return pflag.NormalizedName(name)
	})
	rootCmd.SetUsageTemplate(usageTemplate)
}

func Execute() {
	if err := rootCmd.ExecuteContext(registry.GetContextWithOptions()); err != nil {
		if registry.GetExitCode() == 0 {
			registry.SetExitCode(define.ExecErrorCodeGeneric)
		}
		if registry.IsRemote() {
			if strings.Contains(err.Error(), "unable to connect to Podman") {
				fmt.Fprintln(os.Stderr, "Cannot connect to Podman. Please verify your connection to the Linux system using `podman system connection list`, or try `podman machine init` and `podman machine start` to manage a new Linux VM")
			}
		}
		fmt.Fprintln(os.Stderr, formatError(err))
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
	if cfg.NoOut {
		null, _ := os.Open(os.DevNull)
		os.Stdout = null
	}

	// Currently it is only possible to restore a container with the same runtime
	// as used for checkpointing. It should be possible to make crun and runc
	// compatible to restore a container with another runtime then checkpointed.
	// Currently that does not work.
	// To make it easier for users we will look into the checkpoint archive and
	// set the runtime to the one used during checkpointing.
	if !registry.IsRemote() && cmd.Name() == "restore" {
		if cmd.Flag("import").Changed {
			runtime, err := crutils.CRGetRuntimeFromArchive(cmd.Flag("import").Value.String())
			if err != nil {
				return fmt.Errorf(
					"failed extracting runtime information from %s: %w",
					cmd.Flag("import").Value.String(), err,
				)
			}

			runtimeFlag := cmd.Root().Flag("runtime")
			if runtimeFlag == nil {
				return errors.New("failed to load --runtime flag")
			}

			if !runtimeFlag.Changed {
				// If the user did not select a runtime, this takes the one from
				// the checkpoint archives and tells Podman to use it for the restore.
				if err := runtimeFlag.Value.Set(*runtime); err != nil {
					return err
				}
				runtimeFlag.Changed = true
				logrus.Debugf("Checkpoint was created using '%s'. Restore will use the same runtime", *runtime)
			} else if cfg.RuntimePath != *runtime {
				// If the user selected a runtime on the command-line this checks if
				// it is the same then during checkpointing and errors out if not.
				return fmt.Errorf(
					"checkpoint archive %s was created with runtime '%s' and cannot be restored with runtime '%s'",
					cmd.Flag("import").Value.String(),
					*runtime,
					cfg.RuntimePath,
				)
			}
		}
	}

	setupConnection := func() error {
		var err error
		cfg.URI, cfg.Identity, cfg.MachineMode, err = cfg.ActiveDestination()
		if err != nil {
			return fmt.Errorf("failed to resolve active destination: %w", err)
		}

		if err := cmd.Root().LocalFlags().Set("url", cfg.URI); err != nil {
			return fmt.Errorf("failed to override --url flag: %w", err)
		}

		if err := cmd.Root().LocalFlags().Set("identity", cfg.Identity); err != nil {
			return fmt.Errorf("failed to override --identity flag: %w", err)
		}
		return nil
	}

	// --connection is not as "special" as --remote so we can wait and process it here
	contextConn := cmd.Root().LocalFlags().Lookup("context")
	conn := cmd.Root().LocalFlags().Lookup("connection")
	if conn != nil && conn.Changed {
		if contextConn != nil && contextConn.Changed {
			return fmt.Errorf("use of --connection and --context at the same time is not allowed")
		}
		cfg.Engine.ActiveService = conn.Value.String()
		if err := setupConnection(); err != nil {
			return err
		}
	}
	if contextConn != nil && contextConn.Changed {
		service := contextConn.Value.String()
		if service != "default" {
			cfg.Engine.ActiveService = service
			if err := setupConnection(); err != nil {
				return err
			}
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

	// Hard code TMPDIR functions to use /var/tmp, if user did not override
	if _, ok := os.LookupEnv("TMPDIR"); !ok {
		if tmpdir, err := cfg.ImageCopyTmpDir(); err != nil {
			logrus.Warnf("Failed to retrieve default tmp dir: %s", err.Error())
		} else {
			os.Setenv("TMPDIR", tmpdir)
		}
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
		if cmd.Flag("memory-profile").Changed {
			// Same value as the default in github.com/pkg/profile.
			runtime.MemProfileRate = 4096
			if rate := os.Getenv("MemProfileRate"); rate != "" {
				r, err := strconv.Atoi(rate)
				if err != nil {
					return err
				}
				runtime.MemProfileRate = r
			}
		}

		if cfg.MaxWorks <= 0 {
			return fmt.Errorf("maximum workers must be set to a positive number (got %d)", cfg.MaxWorks)
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
	if !registry.IsRemote() && !found {
		_, noMoveProcess := cmd.Annotations[registry.NoMoveProcess]
		err := registry.ContainerEngine().SetupRootless(registry.Context(), noMoveProcess)
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

	registry.ImageEngine().Shutdown(registry.Context())
	registry.ContainerEngine().Shutdown(registry.Context())

	if registry.IsRemote() {
		return nil
	}

	// CPU and memory profiling.
	if cmd.Flag("cpu-profile").Changed {
		pprof.StopCPUProfile()
	}
	if cmd.Flag("memory-profile").Changed {
		f, err := os.Create(registry.PodmanConfig().MemoryProfile)
		if err != nil {
			return fmt.Errorf("creating memory profile: %w", err)
		}
		defer f.Close()
		runtime.GC() // get up-to-date GC statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			return fmt.Errorf("writing memory profile: %w", err)
		}
	}

	return nil
}

func configHook() {
	if dockerConfig != "" {
		logrus.Warn("The --config flag is ignored by Podman. Exists for Docker compatibility")
	}
}

func loggingHook() {
	var found bool
	if debug {
		if logLevel != defaultLogLevel {
			fmt.Fprintf(os.Stderr, "Setting --log-level and --debug is not allowed\n")
			os.Exit(1)
		}
		logLevel = "debug"
	}
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
	srv, uri, ident, machine := resolveDestination()

	lFlags := cmd.Flags()

	// non configurable option to help ssh dialing
	opts.MachineMode = machine

	sshFlagName := "ssh"
	lFlags.StringVar(&opts.SSHMode, sshFlagName, string(ssh.GolangMode), "define the ssh mode")
	_ = cmd.RegisterFlagCompletionFunc(sshFlagName, common.AutocompleteSSH)

	connectionFlagName := "connection"
	lFlags.StringP(connectionFlagName, "c", srv, "Connection to use for remote Podman service")
	_ = cmd.RegisterFlagCompletionFunc(connectionFlagName, common.AutocompleteSystemConnections)

	urlFlagName := "url"
	lFlags.StringVar(&opts.URI, urlFlagName, uri, "URL to access Podman service (CONTAINER_HOST)")
	_ = cmd.RegisterFlagCompletionFunc(urlFlagName, completion.AutocompleteDefault)
	lFlags.StringVarP(&opts.URI, "host", "H", uri, "Used for Docker compatibility")
	_ = lFlags.MarkHidden("host")

	lFlags.StringVar(&dockerConfig, "config", "", "Ignored for Docker compatibility")
	_ = lFlags.MarkHidden("config")
	// Context option added just for compatibility with DockerCLI.
	lFlags.String("context", "default", "Name of the context to use to connect to the daemon (This flag is a NOOP and provided solely for scripting compatibility.)")
	_ = lFlags.MarkHidden("context")

	identityFlagName := "identity"
	lFlags.StringVar(&opts.Identity, identityFlagName, ident, "path to SSH identity file, (CONTAINER_SSHKEY)")
	_ = cmd.RegisterFlagCompletionFunc(identityFlagName, completion.AutocompleteDefault)

	lFlags.BoolVar(&opts.NoOut, "noout", false, "do not output to stdout")
	lFlags.BoolVarP(&opts.Remote, "remote", "r", registry.IsRemote(), "Access remote Podman service")
	pFlags := cmd.PersistentFlags()
	if registry.IsRemote() {
		if err := lFlags.MarkHidden("remote"); err != nil {
			logrus.Warnf("Unable to mark --remote flag as hidden: %s", err.Error())
		}
		opts.Remote = true
	} else {
		cgroupManagerFlagName := "cgroup-manager"
		pFlags.StringVar(&cfg.Engine.CgroupManager, cgroupManagerFlagName, cfg.Engine.CgroupManager, "Cgroup manager to use (\"cgroupfs\"|\"systemd\")")
		_ = cmd.RegisterFlagCompletionFunc(cgroupManagerFlagName, common.AutocompleteCgroupManager)

		pFlags.StringVar(&opts.CPUProfile, "cpu-profile", "", "Path for the cpu-profiling results")
		pFlags.StringVar(&opts.MemoryProfile, "memory-profile", "", "Path for the memory-profiling results")

		conmonFlagName := "conmon"
		pFlags.StringVar(&opts.ConmonPath, conmonFlagName, "", "Path of the conmon binary")
		_ = cmd.RegisterFlagCompletionFunc(conmonFlagName, completion.AutocompleteDefault)

		networkCmdPathFlagName := "network-cmd-path"
		pFlags.StringVar(&cfg.Engine.NetworkCmdPath, networkCmdPathFlagName, cfg.Engine.NetworkCmdPath, "Path to the command for configuring the network")
		_ = cmd.RegisterFlagCompletionFunc(networkCmdPathFlagName, completion.AutocompleteDefault)

		networkConfigDirFlagName := "network-config-dir"
		pFlags.StringVar(&cfg.Network.NetworkConfigDir, networkConfigDirFlagName, cfg.Network.NetworkConfigDir, "Path of the configuration directory for networks")
		_ = cmd.RegisterFlagCompletionFunc(networkConfigDirFlagName, completion.AutocompleteDefault)

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

		networkBackendFlagName := "network-backend"
		pFlags.StringVar(&cfg.Network.NetworkBackend, networkBackendFlagName, cfg.Network.NetworkBackend, `Network backend to use ("cni"|"netavark")`)
		_ = cmd.RegisterFlagCompletionFunc(networkBackendFlagName, common.AutocompleteNetworkBackend)
		_ = pFlags.MarkHidden(networkBackendFlagName)

		rootFlagName := "root"
		pFlags.StringVar(&cfg.Engine.StaticDir, rootFlagName, "", "Path to the root directory in which data, including images, is stored")
		_ = cmd.RegisterFlagCompletionFunc(rootFlagName, completion.AutocompleteDefault)

		pFlags.StringVar(&opts.RegistriesConf, "registries-conf", "", "Path to a registries.conf to use for image processing")

		runrootFlagName := "runroot"
		pFlags.StringVar(&opts.Runroot, runrootFlagName, "", "Path to the 'run directory' where all state information is stored")
		_ = cmd.RegisterFlagCompletionFunc(runrootFlagName, completion.AutocompleteDefault)

		runtimeFlagName := "runtime"
		pFlags.StringVar(&opts.RuntimePath, runtimeFlagName, cfg.Engine.OCIRuntime, "Path to the OCI-compatible binary used to run containers.")
		_ = cmd.RegisterFlagCompletionFunc(runtimeFlagName, completion.AutocompleteDefault)

		// -s is deprecated due to conflict with -s on subcommands
		storageDriverFlagName := "storage-driver"
		pFlags.StringVar(&opts.StorageDriver, storageDriverFlagName, "", "Select which storage driver is used to manage storage of images and containers")
		_ = cmd.RegisterFlagCompletionFunc(storageDriverFlagName, completion.AutocompleteNone)

		tmpdirFlagName := "tmpdir"
		pFlags.StringVar(&opts.Engine.TmpDir, tmpdirFlagName, "", "Path to the tmp directory for libpod state content.\n\nNote: use the environment variable 'TMPDIR' to change the temporary storage location for container images, '/var/tmp'.\n")
		_ = cmd.RegisterFlagCompletionFunc(tmpdirFlagName, completion.AutocompleteDefault)

		pFlags.BoolVar(&opts.Trace, "trace", false, "Enable opentracing output (default false)")

		volumePathFlagName := "volumepath"
		pFlags.StringVar(&opts.Engine.VolumePath, volumePathFlagName, "", "Path to the volume directory in which volume data is stored")
		_ = cmd.RegisterFlagCompletionFunc(volumePathFlagName, completion.AutocompleteDefault)

		// Hide these flags for both ABI and Tunneling
		for _, f := range []string{
			"cpu-profile",
			"default-mounts-file",
			"max-workers",
			"memory-profile",
			"registries-conf",
			"trace",
		} {
			if err := pFlags.MarkHidden(f); err != nil {
				logrus.Warnf("Unable to mark %s flag as hidden: %s", f, err.Error())
			}
		}
	}
	storageOptFlagName := "storage-opt"
	pFlags.StringArrayVar(&opts.StorageOpts, storageOptFlagName, []string{}, "Used to pass an option to the storage driver")
	_ = cmd.RegisterFlagCompletionFunc(storageOptFlagName, completion.AutocompleteNone)

	// Override default --help information of `--help` global flag
	var dummyHelp bool
	pFlags.BoolVar(&dummyHelp, "help", false, "Help for podman")

	logLevelFlagName := "log-level"
	pFlags.StringVar(&logLevel, logLevelFlagName, logLevel, fmt.Sprintf("Log messages above specified level (%s)", strings.Join(common.LogLevels, ", ")))
	_ = rootCmd.RegisterFlagCompletionFunc(logLevelFlagName, common.AutocompleteLogLevel)

	lFlags.BoolVarP(&debug, "debug", "D", false, "Docker compatibility, force setting of log-level")
	_ = lFlags.MarkHidden("debug")

	// Only create these flags for ABI connections
	if !registry.IsRemote() {
		runtimeflagFlagName := "runtime-flag"
		pFlags.StringArrayVar(&opts.RuntimeFlags, runtimeflagFlagName, []string{}, "add global flags for the container runtime")
		_ = rootCmd.RegisterFlagCompletionFunc(runtimeflagFlagName, completion.AutocompleteNone)

		pFlags.BoolVar(&useSyslog, "syslog", false, "Output logging information to syslog as well as the console (default false)")
	}
}

func resolveDestination() (string, string, string, bool) {
	if uri, found := os.LookupEnv("CONTAINER_HOST"); found {
		var ident string
		if v, found := os.LookupEnv("CONTAINER_SSHKEY"); found {
			ident = v
		}
		return "", uri, ident, false
	}

	cfg, err := config.ReadCustomConfig()
	if err != nil {
		logrus.Warning(fmt.Errorf("unable to read local containers.conf: %w", err))
		return "", registry.DefaultAPIAddress(), "", false
	}

	uri, ident, machine, err := cfg.ActiveDestination()
	if err != nil {
		return "", registry.DefaultAPIAddress(), "", false
	}
	return cfg.Engine.ActiveService, uri, ident, machine
}

func formatError(err error) string {
	var message string
	if errors.Is(err, define.ErrOCIRuntime) {
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
