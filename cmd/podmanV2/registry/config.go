package registry

import (
	"os"
	"runtime"
	"strings"

	"github.com/containers/common/pkg/config"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/sirupsen/logrus"
)

const (
	RootRequired = "RootRequired"
)

var (
	PodmanOptions entities.PodmanConfig
)

// NewPodmanConfig creates a PodmanConfig from the environment
func NewPodmanConfig() entities.PodmanConfig {
	if err := libpod.SetXdgDirs(); err != nil {
		logrus.Errorf(err.Error())
		os.Exit(1)
	}

	var mode entities.EngineMode
	switch runtime.GOOS {
	case "darwin":
		fallthrough
	case "windows":
		mode = entities.TunnelMode
	case "linux":
		mode = entities.ABIMode
	default:
		logrus.Errorf("%s is not a supported OS", runtime.GOOS)
		os.Exit(1)
	}

	// cobra.Execute() may not be called yet, so we peek at os.Args.
	for _, v := range os.Args {
		// Prefix checking works because of how default EngineMode's
		// have been defined.
		if strings.HasPrefix(v, "--remote=") {
			mode = entities.TunnelMode
		}
	}

	// FIXME: for rootless, where to get the path
	// TODO:
	cfg, err := config.NewConfig("")
	if err != nil {
		logrus.Error("Failed to obtain podman configuration")
		os.Exit(1)
	}
	return entities.PodmanConfig{Config: cfg, EngineMode: mode}
}
