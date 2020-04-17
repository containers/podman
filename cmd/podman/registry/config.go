package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/containers/common/pkg/config"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
)

const (
	ParentNSRequired = "ParentNSRequired"
)

var (
	podmanOptions entities.PodmanConfig
	podmanSync    sync.Once
)

// PodmanConfig returns an entities.PodmanConfig built up from
// environment and CLI
func PodmanConfig() *entities.PodmanConfig {
	podmanSync.Do(newPodmanConfig)
	return &podmanOptions
}

func newPodmanConfig() {
	if err := setXdgDirs(); err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
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
		fmt.Fprintf(os.Stderr, "%s is not a supported OS", runtime.GOOS)
		os.Exit(1)
	}

	// cobra.Execute() may not be called yet, so we peek at os.Args.
	for _, v := range os.Args {
		// Prefix checking works because of how default EngineMode's
		// have been defined.
		if strings.HasPrefix(v, "--remote") {
			mode = entities.TunnelMode
		}
	}

	// FIXME: for rootless, add flag to get the path to override configuration
	cfg, err := config.NewConfig("")
	if err != nil {
		fmt.Fprint(os.Stderr, "Failed to obtain podman configuration: "+err.Error())
		os.Exit(1)
	}

	cfg.Network.NetworkConfigDir = cfg.Network.CNIPluginDirs[0]
	if rootless.IsRootless() {
		cfg.Network.NetworkConfigDir = ""
	}

	podmanOptions = entities.PodmanConfig{Config: cfg, EngineMode: mode}
}

// SetXdgDirs ensures the XDG_RUNTIME_DIR env and XDG_CONFIG_HOME variables are set.
// containers/image uses XDG_RUNTIME_DIR to locate the auth file, XDG_CONFIG_HOME is
// use for the libpod.conf configuration file.
func setXdgDirs() error {
	if !rootless.IsRootless() {
		return nil
	}

	// Setup XDG_RUNTIME_DIR
	if _, found := os.LookupEnv("XDG_RUNTIME_DIR"); !found {
		dir, err := util.GetRuntimeDir()
		if err != nil {
			return err
		}
		if err := os.Setenv("XDG_RUNTIME_DIR", dir); err != nil {
			return errors.Wrapf(err, "cannot set XDG_RUNTIME_DIR="+dir)
		}
	}

	if _, found := os.LookupEnv("DBUS_SESSION_BUS_ADDRESS"); !found {
		sessionAddr := filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "bus")
		if _, err := os.Stat(sessionAddr); err == nil {
			os.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+sessionAddr)
		}
	}

	// Setup XDG_CONFIG_HOME
	if _, found := os.LookupEnv("XDG_CONFIG_HOME"); !found {
		cfgHomeDir, err := util.GetRootlessConfigHomeDir()
		if err != nil {
			return err
		}
		if err := os.Setenv("XDG_CONFIG_HOME", cfgHomeDir); err != nil {
			return errors.Wrapf(err, "cannot set XDG_CONFIG_HOME="+cfgHomeDir)
		}
	}
	return nil
}
