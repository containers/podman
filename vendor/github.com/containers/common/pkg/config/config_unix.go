//go:build !windows

package config

import (
	"os"
	"path/filepath"

	"github.com/containers/storage/pkg/unshare"
)

// userConfigPath returns the path to the users local config that is
// not shared with other users. It uses $XDG_CONFIG_HOME/containers...
// if set or $HOME/.config/containers... if not.
func userConfigPath() (string, error) {
	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		return filepath.Join(configHome, _configPath), nil
	}
	home, err := unshare.HomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, UserOverrideContainersConfig), nil
}
