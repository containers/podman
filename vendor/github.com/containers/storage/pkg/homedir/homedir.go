package homedir

import (
	"errors"
	"os"
	"path/filepath"
)

// GetConfigHome returns XDG_CONFIG_HOME.
// GetConfigHome returns $HOME/.config and nil error if XDG_CONFIG_HOME is not set.
//
// See also https://standards.freedesktop.org/basedir-spec/latest/ar01s03.html
func GetConfigHome() (string, error) {
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		return xdgConfigHome, nil
	}
	home := Get()
	if home == "" {
		return "", errors.New("could not get either XDG_CONFIG_HOME or HOME")
	}
	return filepath.Join(home, ".config"), nil
}

// GetDataHome returns XDG_DATA_HOME.
// GetDataHome returns $HOME/.local/share and nil error if XDG_DATA_HOME is not set.
//
// See also https://standards.freedesktop.org/basedir-spec/latest/ar01s03.html
func GetDataHome() (string, error) {
	if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
		return xdgDataHome, nil
	}
	home := Get()
	if home == "" {
		return "", errors.New("could not get either XDG_DATA_HOME or HOME")
	}
	return filepath.Join(home, ".local", "share"), nil
}

// GetCacheHome returns XDG_CACHE_HOME.
// GetCacheHome returns $HOME/.cache and nil error if XDG_CACHE_HOME is not set.
//
// See also https://standards.freedesktop.org/basedir-spec/latest/ar01s03.html
func GetCacheHome() (string, error) {
	if xdgCacheHome := os.Getenv("XDG_CACHE_HOME"); xdgCacheHome != "" {
		return xdgCacheHome, nil
	}
	home := Get()
	if home == "" {
		return "", errors.New("could not get either XDG_CACHE_HOME or HOME")
	}
	return filepath.Join(home, ".cache"), nil
}
