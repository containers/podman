package config

import (
	"io/fs"
	"os"
	"path/filepath"
)

const (
	// _configPath is the path to the containers/containers.conf
	// inside a given config directory.
	_configPath = "\\containers\\containers.conf"

	// defaultContainersConfig holds the default containers config path
	defaultContainersConfig = ""

	// DefaultSignaturePolicyPath is the default value for the
	// policy.json file.
	DefaultSignaturePolicyPath = "/etc/containers/policy.json"

	// Mount type for mounting host dir
	_typeBind = "bind"
)

// userConfigPath returns the path to the users local config that is
// not shared with other users. It uses $APPDATA/containers...
func userConfigPath() (string, error) {
	return os.Getenv("APPDATA") + _configPath, nil
}

// overrideContainersConfigPath returns the path to the system wide
// containers config folder. It users $PROGRAMDATA/containers...
func overrideContainersConfigPath() (string, error) {
	return os.Getenv("ProgramData") + _configPath, nil
}

var defaultHelperBinariesDir = []string{
	// FindHelperBinaries(), as a convention, interprets $BINDIR as the
	// directory where the current process binary (i.e. podman) is located.
	"$BINDIR",
}

func safeEvalSymlinks(filePath string) (string, error) {
	fileInfo, err := os.Lstat(filePath)
	if err != nil {
		return "", err
	}
	if fileInfo.Mode()&fs.ModeSymlink != 0 {
		// Only call filepath.EvalSymlinks if it is a symlink.
		// Starting with v1.23, EvalSymlinks returns an error for mount points.
		// See https://go-review.googlesource.com/c/go/+/565136 for reference.
		filePath, err = filepath.EvalSymlinks(filePath)
		if err != nil {
			return "", err
		}
	} else {
		// Call filepath.Clean when filePath is not a symlink. That's for
		// consistency with the symlink case (filepath.EvalSymlinks calls
		// Clean after evaluating filePath).
		filePath = filepath.Clean(filePath)
	}
	return filePath, nil
}
