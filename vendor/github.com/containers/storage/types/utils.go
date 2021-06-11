package types

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containers/storage/pkg/homedir"
	"github.com/containers/storage/pkg/system"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// GetRootlessRuntimeDir returns the runtime directory when running as non root
func GetRootlessRuntimeDir(rootlessUID int) (string, error) {
	path, err := getRootlessRuntimeDir(rootlessUID)
	if err != nil {
		return "", err
	}
	path = filepath.Join(path, "containers")
	if err := os.MkdirAll(path, 0700); err != nil {
		return "", errors.Wrapf(err, "unable to make rootless runtime")
	}
	return path, nil
}

type rootlessRuntimeDirEnvironment interface {
	getProcCommandFile() string
	getRunUserDir() string
	getTmpPerUserDir() string

	homeDirGetRuntimeDir() (string, error)
	systemLstat(string) (*system.StatT, error)
	homedirGet() string
}

type rootlessRuntimeDirEnvironmentImplementation struct {
	procCommandFile string
	runUserDir      string
	tmpPerUserDir   string
}

func (env rootlessRuntimeDirEnvironmentImplementation) getProcCommandFile() string {
	return env.procCommandFile
}
func (env rootlessRuntimeDirEnvironmentImplementation) getRunUserDir() string {
	return env.runUserDir
}
func (env rootlessRuntimeDirEnvironmentImplementation) getTmpPerUserDir() string {
	return env.tmpPerUserDir
}
func (rootlessRuntimeDirEnvironmentImplementation) homeDirGetRuntimeDir() (string, error) {
	return homedir.GetRuntimeDir()
}
func (rootlessRuntimeDirEnvironmentImplementation) systemLstat(path string) (*system.StatT, error) {
	return system.Lstat(path)
}
func (rootlessRuntimeDirEnvironmentImplementation) homedirGet() string {
	return homedir.Get()
}

func isRootlessRuntimeDirOwner(dir string, env rootlessRuntimeDirEnvironment) bool {
	st, err := env.systemLstat(dir)
	return err == nil && int(st.UID()) == os.Getuid() && st.Mode()&0700 == 0700 && st.Mode()&0066 == 0000
}

// getRootlessRuntimeDirIsolated is an internal implementation detail of getRootlessRuntimeDir to allow testing.
// Everyone but the tests this is intended for should only call getRootlessRuntimeDir, never this function.
func getRootlessRuntimeDirIsolated(env rootlessRuntimeDirEnvironment) (string, error) {
	runtimeDir, err := env.homeDirGetRuntimeDir()
	if err == nil {
		return runtimeDir, nil
	}

	initCommand, err := ioutil.ReadFile(env.getProcCommandFile())
	if err != nil || string(initCommand) == "systemd" {
		runUserDir := env.getRunUserDir()
		if isRootlessRuntimeDirOwner(runUserDir, env) {
			return runUserDir, nil
		}
	}

	tmpPerUserDir := env.getTmpPerUserDir()
	if tmpPerUserDir != "" {
		if _, err := env.systemLstat(tmpPerUserDir); os.IsNotExist(err) {
			if err := os.Mkdir(tmpPerUserDir, 0700); err != nil {
				logrus.Errorf("failed to create temp directory for user: %v", err)
			} else {
				return tmpPerUserDir, nil
			}
		} else if isRootlessRuntimeDirOwner(tmpPerUserDir, env) {
			return tmpPerUserDir, nil
		}
	}

	homeDir := env.homedirGet()
	if homeDir == "" {
		return "", errors.New("neither XDG_RUNTIME_DIR nor temp dir nor HOME was set non-empty")
	}
	resolvedHomeDir, err := filepath.EvalSymlinks(homeDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedHomeDir, "rundir"), nil
}

func getRootlessRuntimeDir(rootlessUID int) (string, error) {
	return getRootlessRuntimeDirIsolated(
		rootlessRuntimeDirEnvironmentImplementation{
			"/proc/1/comm",
			fmt.Sprintf("/run/user/%d", rootlessUID),
			fmt.Sprintf("%s/containers-user-%d", os.TempDir(), rootlessUID),
		},
	)
}

// getRootlessDirInfo returns the parent path of where the storage for containers and
// volumes will be in rootless mode
func getRootlessDirInfo(rootlessUID int) (string, string, error) {
	rootlessRuntime, err := GetRootlessRuntimeDir(rootlessUID)
	if err != nil {
		return "", "", err
	}

	dataDir, err := homedir.GetDataHome()
	if err == nil {
		return dataDir, rootlessRuntime, nil
	}

	home := homedir.Get()
	if home == "" {
		return "", "", errors.Wrapf(err, "neither XDG_DATA_HOME nor HOME was set non-empty")
	}
	// runc doesn't like symlinks in the rootfs path, and at least
	// on CoreOS /home is a symlink to /var/home, so resolve any symlink.
	resolvedHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		return "", "", err
	}
	dataDir = filepath.Join(resolvedHome, ".local", "share")

	return dataDir, rootlessRuntime, nil
}

func getRootlessUID() int {
	uidEnv := os.Getenv("_CONTAINERS_ROOTLESS_UID")
	if uidEnv != "" {
		u, _ := strconv.Atoi(uidEnv)
		return u
	}
	return os.Geteuid()
}

func expandEnvPath(path string, rootlessUID int) (string, error) {
	path = strings.Replace(path, "$UID", strconv.Itoa(rootlessUID), -1)
	return filepath.Clean(os.ExpandEnv(path)), nil
}

func DefaultConfigFile(rootless bool) (string, error) {
	if defaultConfigFileSet {
		return defaultConfigFile, nil
	}

	if path, ok := os.LookupEnv("CONTAINERS_STORAGE_CONF"); ok {
		return path, nil
	}
	if !rootless {
		return defaultConfigFile, nil
	}

	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		return filepath.Join(configHome, "containers/storage.conf"), nil
	}
	home := homedir.Get()
	if home == "" {
		return "", errors.New("cannot determine user's homedir")
	}
	return filepath.Join(home, ".config/containers/storage.conf"), nil
}

func reloadConfigurationFileIfNeeded(configFile string, storeOptions *StoreOptions) {
	prevReloadConfig.mutex.Lock()
	defer prevReloadConfig.mutex.Unlock()

	fi, err := os.Stat(configFile)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("Failed to read %s %v\n", configFile, err.Error())
		}
		return
	}

	mtime := fi.ModTime()
	if prevReloadConfig.storeOptions != nil && prevReloadConfig.mod == mtime && prevReloadConfig.configFile == configFile {
		*storeOptions = *prevReloadConfig.storeOptions
		return
	}

	ReloadConfigurationFile(configFile, storeOptions)

	prevReloadConfig.storeOptions = storeOptions
	prevReloadConfig.mod = mtime
	prevReloadConfig.configFile = configFile
}
