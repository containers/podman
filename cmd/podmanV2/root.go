package main

import (
	"fmt"
	"log/syslog"
	"os"
	"path"

	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/version"
	"github.com/sirupsen/logrus"
	logrusSyslog "github.com/sirupsen/logrus/hooks/syslog"
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:               path.Base(os.Args[0]),
		Long:              "Manage pods, containers and images",
		SilenceUsage:      true,
		SilenceErrors:     true,
		TraverseChildren:  true,
		PersistentPreRunE: preRunE,
		RunE:              registry.SubCommandExists,
		Version:           version.Version,
	}

	logLevels = entities.NewStringSet("debug", "info", "warn", "error", "fatal", "panic")
	logLevel  = "error"
	useSyslog bool
)

func init() {
	// Override default --help information of `--version` global flag}
	var dummyVersion bool
	// TODO had to disable shorthand -v for version due to -v rm with volume
	rootCmd.PersistentFlags().BoolVar(&dummyVersion, "version", false, "Version of Podman")
	rootCmd.PersistentFlags().StringVarP(&registry.EngineOptions.Uri, "remote", "r", "", "URL to access Podman service")
	rootCmd.PersistentFlags().StringSliceVar(&registry.EngineOptions.Identities, "identity", []string{}, "path to SSH identity file")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "error", fmt.Sprintf("Log messages above specified level (%s)", logLevels.String()))
	rootCmd.PersistentFlags().BoolVar(&useSyslog, "syslog", false, "Output logging information to syslog as well as the console (default false)")

	cobra.OnInitialize(
		logging,
		syslogHook,
	)
}

func preRunE(cmd *cobra.Command, args []string) error {
	cmd.SetHelpTemplate(registry.HelpTemplate())
	cmd.SetUsageTemplate(registry.UsageTemplate())
	return nil
}

func logging() {
	if !logLevels.Contains(logLevel) {
		fmt.Fprintf(os.Stderr, "Log Level \"%s\" is not supported, choose from: %s\n", logLevel, logLevels.String())
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

func Execute() {
	o := registry.NewOptions(rootCmd.Context(), &registry.EngineOptions)
	if err := rootCmd.ExecuteContext(o); err != nil {
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
