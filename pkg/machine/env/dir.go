package env

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/storage/pkg/fileutils"
	"go.podman.io/storage/pkg/homedir"
)

var getToolName = sync.OnceValue(func() string {
	toolName := os.Getenv("PODMAN_TOOL_PREFIX")
	if toolName == "" {
		toolName = "podman"
	}
	return toolName
})

// GetDataDir returns the filepath where vm images should
// live for podman-machine.
func GetDataDir(vmType define.VMType) (string, error) {
	dataDirPrefix, err := DataDirPrefix()
	if err != nil {
		return "", err
	}
	dataDir := filepath.Join(dataDirPrefix, vmType.String())
	if err := fileutils.Exists(dataDir); !errors.Is(err, os.ErrNotExist) {
		return dataDir, nil
	}
	mkdirErr := os.MkdirAll(dataDir, 0o755)
	return dataDir, mkdirErr
}

// GetGlobalDataDir returns the root of all backends
// for shared machine data.
func GetGlobalDataDir() (string, error) {
	dataDir, err := DataDirPrefix()
	if err != nil {
		return "", err
	}

	return dataDir, os.MkdirAll(dataDir, 0o755)
}

func GetMachineDirs(vmType define.VMType) (*define.MachineDirs, error) {
	rtDir, err := getRuntimeDir()
	if err != nil {
		return nil, err
	}

	// The runtime directory can be customized using the PODMAN_RUNTIME_DIR env variable
	// Its default value will be podman
	// When used by macadam it can be customized to use a different path like macadam
	runtimeDir := GetRuntimeDirSuffix()
	rtDir = filepath.Join(rtDir, runtimeDir)
	configDir, err := GetConfDir(vmType)
	if err != nil {
		return nil, err
	}

	configDirFile, err := define.NewMachineFile(configDir, nil)
	if err != nil {
		return nil, err
	}
	dataDir, err := GetDataDir(vmType)
	if err != nil {
		return nil, err
	}

	dataDirFile, err := define.NewMachineFile(dataDir, nil)
	if err != nil {
		return nil, err
	}

	imageCacheDir, err := dataDirFile.AppendToNewVMFile("cache", nil)
	if err != nil {
		return nil, err
	}

	rtDirFile, err := define.NewMachineFile(rtDir, nil)
	if err != nil {
		return nil, err
	}

	dirs := define.MachineDirs{
		ConfigDir:     configDirFile,
		DataDir:       dataDirFile,
		ImageCacheDir: imageCacheDir,
		RuntimeDir:    rtDirFile,
	}

	// make sure all machine dirs are present
	if err := os.MkdirAll(rtDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return nil, err
	}

	// Because this is a mkdirall, we make the image cache dir
	// which is a subdir of datadir (so the datadir is made anyway)
	err = os.MkdirAll(imageCacheDir.GetPath(), 0o755)

	return &dirs, err
}

// DataDirPrefix returns the path prefix for all machine data files
// The config directory can be customized using the PODMAN_DATA_DIR env variable
// Its default value will be /podman/machine
// When used by macadam it can be customized to use a different path like /macadam/machine
func DataDirPrefix() (string, error) {
	data, err := homedir.GetDataHome()
	if err != nil {
		return "", err
	}
	machineDataDir := getDataDirSuffix()
	dataDir := filepath.Join(data, "containers", machineDataDir)
	return dataDir, nil
}

// GetConfigDir returns the filepath to where configuration
// files for podman-machine should live
func GetConfDir(vmType define.VMType) (string, error) {
	confDirPrefix, err := ConfDirPrefix()
	if err != nil {
		return "", err
	}
	confDir := filepath.Join(confDirPrefix, vmType.String())
	if err := fileutils.Exists(confDir); !errors.Is(err, os.ErrNotExist) {
		return confDir, nil
	}
	mkdirErr := os.MkdirAll(confDir, 0o755)
	return confDir, mkdirErr
}

// ConfDirPrefix returns the path prefix for all machine config files
// The config directory can be customized using the PODMAN_DATA_DIR env variable
// Its default value will be /podman/machine
// When used by macadam it can be customized to use a different path like /macadam/machine
func ConfDirPrefix() (string, error) {
	conf, err := homedir.GetConfigHome()
	if err != nil {
		return "", err
	}
	machineDataDir := getDataDirSuffix()
	confDir := filepath.Join(conf, "containers", machineDataDir)
	return confDir, nil
}

// GetSSHIdentityPath returns the path to the expected SSH private key
func GetSSHIdentityPath(name string) (string, error) {
	datadir, err := GetGlobalDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(datadir, name), nil
}

func WithToolPrefix(name string) string {
	toolName := getToolName()
	if !strings.HasPrefix(name, toolName) {
		name = fmt.Sprintf("%s-%s", toolName, name)
	}
	return name
}

func getDataDirSuffix() string {
	machineDir := os.Getenv("PODMAN_DATA_DIR")
	if machineDir == "" {
		machineDir = filepath.Join("podman", "machine")
	}
	return machineDir
}

func GetRuntimeDirSuffix() string {
	runtimeDir := os.Getenv("PODMAN_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = filepath.Join("podman")
	}
	return runtimeDir
}
