package registry

import (
	"context"
	"path/filepath"

	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/domain/infra"
	"github.com/containers/podman/v5/pkg/rootless"
	"github.com/containers/podman/v5/pkg/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// DefaultRootAPIAddress is the default path of the REST socket with unix:// prefix
const DefaultRootAPIAddress = "unix://" + DefaultRootAPIPath

type CliCommand struct {
	Command *cobra.Command
	Parent  *cobra.Command
}

var (
	cliCtx          context.Context
	containerEngine entities.ContainerEngine
	exitCode        = 0
	imageEngine     entities.ImageEngine

	// Commands holds the cobra.Commands to present to the user, including
	// parent if not a child of "root"
	Commands []CliCommand
)

func SetExitCode(code int) {
	exitCode = code
}

func GetExitCode() int {
	return exitCode
}

func ImageEngine() entities.ImageEngine {
	return imageEngine
}

// NewImageEngine is a wrapper for building an ImageEngine to be used for PreRunE functions
func NewImageEngine(cmd *cobra.Command, args []string) (entities.ImageEngine, error) {
	if imageEngine == nil {
		podmanOptions.FlagSet = cmd.Flags()
		engine, err := infra.NewImageEngine(&podmanOptions)
		if err != nil {
			return nil, err
		}
		imageEngine = engine
	}
	return imageEngine, nil
}

func ContainerEngine() entities.ContainerEngine {
	return containerEngine
}

// NewContainerEngine is a wrapper for building a ContainerEngine to be used for PreRunE functions
func NewContainerEngine(cmd *cobra.Command, args []string) (entities.ContainerEngine, error) {
	if containerEngine == nil {
		podmanOptions.FlagSet = cmd.Flags()
		if cmd.Name() == "reset" && cmd.Parent().Name() == "system" {
			logrus.Debugf("Performing system reset, runtime validation checks will be relaxed")
			podmanOptions.IsReset = true
		}
		if cmd.Name() == "renumber" && cmd.Parent().Name() == "system" {
			logrus.Debugf("Performing system renumber, runtime validation checks will be relaxed")
			podmanOptions.IsRenumber = true
		}
		engine, err := infra.NewContainerEngine(&podmanOptions)
		if err != nil {
			return nil, err
		}
		containerEngine = engine
	}
	return containerEngine, nil
}

type PodmanOptionsKey struct{}

func Context() context.Context {
	if cliCtx == nil {
		cliCtx = ContextWithOptions(context.Background())
	}
	return cliCtx
}

func ContextWithOptions(ctx context.Context) context.Context {
	cliCtx = context.WithValue(ctx, PodmanOptionsKey{}, podmanOptions)
	return cliCtx
}

// GetContextWithOptions deprecated, use  NewContextWithOptions()
func GetContextWithOptions() context.Context {
	return ContextWithOptions(context.Background())
}

// GetContext deprecated, use  Context()
func GetContext() context.Context {
	return Context()
}

func DefaultAPIAddress() string {
	if rootless.IsRootless() {
		xdg, err := util.GetRootlessRuntimeDir()
		if err != nil {
			logrus.Warnf("Failed to get rootless runtime dir for DefaultAPIAddress: %s", err)
			return DefaultRootAPIAddress
		}
		return "unix://" + filepath.Join(xdg, "podman", "podman.sock")
	}
	return DefaultRootAPIAddress
}
