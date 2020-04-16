package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containers/common/pkg/config"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
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
	if err := setXdgDirs(); err != nil {
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

// SetXdgDirs ensures the XDG_RUNTIME_DIR env and XDG_CONFIG_HOME variables are set.
// containers/image uses XDG_RUNTIME_DIR to locate the auth file, XDG_CONFIG_HOME is
// use for the libpod.conf configuration file.
func setXdgDirs() error {
	if !rootless.IsRootless() {
		return nil
	}

	// Setup XDG_RUNTIME_DIR
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")

	if runtimeDir == "" {
		var err error
		runtimeDir, err = util.GetRuntimeDir()
		if err != nil {
			return err
		}
	}
	if err := os.Setenv("XDG_RUNTIME_DIR", runtimeDir); err != nil {
		return errors.Wrapf(err, "cannot set XDG_RUNTIME_DIR")
	}

	if rootless.IsRootless() && os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		sessionAddr := filepath.Join(runtimeDir, "bus")
		if _, err := os.Stat(sessionAddr); err == nil {
			os.Setenv("DBUS_SESSION_BUS_ADDRESS", fmt.Sprintf("unix:path=%s", sessionAddr))
		}
	}

	// Setup XDG_CONFIG_HOME
	if cfgHomeDir := os.Getenv("XDG_CONFIG_HOME"); cfgHomeDir == "" {
		cfgHomeDir, err := util.GetRootlessConfigHomeDir()
		if err != nil {
			return err
		}
		if err := os.Setenv("XDG_CONFIG_HOME", cfgHomeDir); err != nil {
			return errors.Wrapf(err, "cannot set XDG_CONFIG_HOME")
		}
	}
	return nil
}
