//go:build !remote

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/containers/common/pkg/config"
	_ "github.com/containers/podman/v5/cmd/podman/completion"
	ientities "github.com/containers/podman/v5/internal/domain/entities"
	"github.com/containers/podman/v5/internal/domain/infra"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/reexec"
	"github.com/containers/storage/pkg/unshare"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	mainCmd = &cobra.Command{
		Use:  "podman-testing",
		Long: "Assorted tools for use in testing podman",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return before()
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			return after()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	mainContext          = context.Background()
	podmanConfig         entities.PodmanConfig
	globalStorageOptions storage.StoreOptions
	globalLogLevel       string
	testingEngine        ientities.TestingEngine
)

func init() {
	podmanConfig.FlagSet = mainCmd.PersistentFlags()
	fl := mainCmd.PersistentFlags()
	fl.StringVar(&podmanConfig.DockerConfig, "docker-config", os.Getenv("DOCKER_CONFIG"), "path to .docker/config")
	fl.StringVar(&globalLogLevel, "log-level", "warn", "logging level")
	fl.StringVar(&podmanConfig.URI, "url", "", "URL to access Podman service")
	fl.StringVar(&podmanConfig.RegistriesConf, "registries-conf", os.Getenv("REGISTRIES_CONF"), "path to registries.conf (REGISTRIES_CONF)")
}

func before() error {
	if globalLogLevel != "" {
		parsedLogLevel, err := logrus.ParseLevel(globalLogLevel)
		if err != nil {
			return fmt.Errorf("parsing log level %q: %w", globalLogLevel, err)
		}
		logrus.SetLevel(parsedLogLevel)
	}
	if err := storeBefore(); err != nil {
		return fmt.Errorf("setting up storage: %w", err)
	}

	podmanConfig.EngineMode = engineMode
	podmanConfig.Remote = podmanConfig.URI != ""

	containersConf, err := config.Default()
	if err != nil {
		return fmt.Errorf("loading default configuration (may reference $CONTAINERS_CONF): %w", err)
	}
	podmanConfig.ContainersConfDefaultsRO = containersConf
	containersConf, err = config.New(nil)
	if err != nil {
		return fmt.Errorf("loading default configuration (may reference $CONTAINERS_CONF): %w", err)
	}
	podmanConfig.ContainersConf = containersConf

	podmanConfig.StorageDriver = globalStorageOptions.GraphDriverName
	podmanConfig.GraphRoot = globalStorageOptions.GraphRoot
	podmanConfig.Runroot = globalStorageOptions.RunRoot
	podmanConfig.ImageStore = globalStorageOptions.ImageStore
	podmanConfig.StorageOpts = globalStorageOptions.GraphDriverOptions
	podmanConfig.TransientStore = globalStorageOptions.TransientStore

	te, err := infra.NewTestingEngine(&podmanConfig)
	if err != nil {
		return fmt.Errorf("initializing libpod: %w", err)
	}
	testingEngine = te
	return nil
}

func after() error {
	if err := storeAfter(); err != nil {
		return fmt.Errorf("shutting down storage: %w", err)
	}
	return nil
}

func main() {
	if reexec.Init() {
		// We were invoked with a different argv[0] indicating that we
		// had a specific job to do as a subprocess, and it's done.
		return
	}
	unshare.MaybeReexecUsingUserNamespace(false)

	exitCode := 1
	if err := mainCmd.Execute(); err != nil {
		if logrus.IsLevelEnabled(logrus.TraceLevel) {
			fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			if w, ok := ee.Sys().(syscall.WaitStatus); ok {
				exitCode = w.ExitStatus()
			}
		}
	} else {
		exitCode = 0
	}
	os.Exit(exitCode)
}
