package storage

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containers/storage/pkg/homedir"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/system"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ParseIDMapping takes idmappings and subuid and subgid maps and returns a storage mapping
func ParseIDMapping(UIDMapSlice, GIDMapSlice []string, subUIDMap, subGIDMap string) (*IDMappingOptions, error) {
	options := IDMappingOptions{
		HostUIDMapping: true,
		HostGIDMapping: true,
	}
	if subGIDMap == "" && subUIDMap != "" {
		subGIDMap = subUIDMap
	}
	if subUIDMap == "" && subGIDMap != "" {
		subUIDMap = subGIDMap
	}
	if len(GIDMapSlice) == 0 && len(UIDMapSlice) != 0 {
		GIDMapSlice = UIDMapSlice
	}
	if len(UIDMapSlice) == 0 && len(GIDMapSlice) != 0 {
		UIDMapSlice = GIDMapSlice
	}
	if len(UIDMapSlice) == 0 && subUIDMap == "" && os.Getuid() != 0 {
		UIDMapSlice = []string{fmt.Sprintf("0:%d:1", os.Getuid())}
	}
	if len(GIDMapSlice) == 0 && subGIDMap == "" && os.Getuid() != 0 {
		GIDMapSlice = []string{fmt.Sprintf("0:%d:1", os.Getgid())}
	}

	if subUIDMap != "" && subGIDMap != "" {
		mappings, err := idtools.NewIDMappings(subUIDMap, subGIDMap)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create NewIDMappings for uidmap=%s gidmap=%s", subUIDMap, subGIDMap)
		}
		options.UIDMap = mappings.UIDs()
		options.GIDMap = mappings.GIDs()
	}
	parsedUIDMap, err := idtools.ParseIDMap(UIDMapSlice, "UID")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create ParseUIDMap UID=%s", UIDMapSlice)
	}
	parsedGIDMap, err := idtools.ParseIDMap(GIDMapSlice, "GID")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create ParseGIDMap GID=%s", UIDMapSlice)
	}
	options.UIDMap = append(options.UIDMap, parsedUIDMap...)
	options.GIDMap = append(options.GIDMap, parsedGIDMap...)
	if len(options.UIDMap) > 0 {
		options.HostUIDMapping = false
	}
	if len(options.GIDMap) > 0 {
		options.HostGIDMapping = false
	}
	return &options, nil
}

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
	if _, err := env.systemLstat(tmpPerUserDir); os.IsNotExist(err) {
		if err := os.Mkdir(tmpPerUserDir, 0700); err != nil {
			logrus.Errorf("failed to create temp directory for user: %v", err)
		} else {
			return tmpPerUserDir, nil
		}
	} else if isRootlessRuntimeDirOwner(tmpPerUserDir, env) {
		return tmpPerUserDir, nil
	}

	homeDir := env.homedirGet()
	if homeDir == "" {
		return "", errors.New("neither XDG_RUNTIME_DIR not temp dir nor HOME was set non-empty")
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

// getRootlessStorageOpts returns the storage opts for containers running as non root
func getRootlessStorageOpts(rootlessUID int, systemOpts StoreOptions) (StoreOptions, error) {
	var opts StoreOptions

	dataDir, rootlessRuntime, err := getRootlessDirInfo(rootlessUID)
	if err != nil {
		return opts, err
	}
	opts.RunRoot = rootlessRuntime
	if systemOpts.RootlessStoragePath != "" {
		opts.GraphRoot = systemOpts.RootlessStoragePath
	} else {
		opts.GraphRoot = filepath.Join(dataDir, "containers", "storage")
	}
	if path, err := exec.LookPath("fuse-overlayfs"); err == nil {
		opts.GraphDriverName = "overlay"
		opts.GraphDriverOptions = []string{fmt.Sprintf("overlay.mount_program=%s", path)}
		for _, o := range systemOpts.GraphDriverOptions {
			if strings.Contains(o, "ignore_chown_errors") {
				opts.GraphDriverOptions = append(opts.GraphDriverOptions, o)
				break
			}
		}
	} else {
		opts.GraphDriverName = "vfs"
	}
	return opts, nil
}

func getRootlessUID() int {
	uidEnv := os.Getenv("_CONTAINERS_ROOTLESS_UID")
	if uidEnv != "" {
		u, _ := strconv.Atoi(uidEnv)
		return u
	}
	return os.Geteuid()
}

// DefaultStoreOptionsAutoDetectUID returns the default storage ops for containers
func DefaultStoreOptionsAutoDetectUID() (StoreOptions, error) {
	uid := getRootlessUID()
	return DefaultStoreOptions(uid != 0, uid)
}

// defaultStoreOptionsIsolated is an internal implementation detail of DefaultStoreOptions to allow testing.
// Everyone but the tests this is intended for should only call DefaultStoreOptions, never this function.
func defaultStoreOptionsIsolated(rootless bool, rootlessUID int, storageConf string) (StoreOptions, error) {
	var (
		defaultRootlessRunRoot   string
		defaultRootlessGraphRoot string
		err                      error
	)
	storageOpts := defaultStoreOptions
	if rootless && rootlessUID != 0 {
		storageOpts, err = getRootlessStorageOpts(rootlessUID, storageOpts)
		if err != nil {
			return storageOpts, err
		}
	}
	_, err = os.Stat(storageConf)
	if err != nil && !os.IsNotExist(err) {
		return storageOpts, err
	}
	if err == nil && !defaultConfigFileSet {
		defaultRootlessRunRoot = storageOpts.RunRoot
		defaultRootlessGraphRoot = storageOpts.GraphRoot
		storageOpts = StoreOptions{}
		reloadConfigurationFileIfNeeded(storageConf, &storageOpts)
		if rootless && rootlessUID != 0 {
			// If the file did not specify a graphroot or runroot,
			// set sane defaults so we don't try and use root-owned
			// directories
			if storageOpts.RunRoot == "" {
				storageOpts.RunRoot = defaultRootlessRunRoot
			}
			if storageOpts.GraphRoot == "" {
				if storageOpts.RootlessStoragePath != "" {
					storageOpts.GraphRoot = storageOpts.RootlessStoragePath
				} else {
					storageOpts.GraphRoot = defaultRootlessGraphRoot
				}
			}
		}
	}
	if storageOpts.RunRoot != "" {
		runRoot, err := expandEnvPath(storageOpts.RunRoot, rootlessUID)
		if err != nil {
			return storageOpts, err
		}
		storageOpts.RunRoot = runRoot
	}
	if storageOpts.GraphRoot != "" {
		graphRoot, err := expandEnvPath(storageOpts.GraphRoot, rootlessUID)
		if err != nil {
			return storageOpts, err
		}
		storageOpts.GraphRoot = graphRoot
	}

	return storageOpts, nil
}

// DefaultStoreOptions returns the default storage ops for containers
func DefaultStoreOptions(rootless bool, rootlessUID int) (StoreOptions, error) {
	storageConf, err := DefaultConfigFile(rootless && rootlessUID != 0)
	if err != nil {
		return defaultStoreOptions, err
	}
	return defaultStoreOptionsIsolated(rootless, rootlessUID, storageConf)
}

func expandEnvPath(path string, rootlessUID int) (string, error) {
	path = strings.Replace(path, "$UID", strconv.Itoa(rootlessUID), -1)
	path = os.ExpandEnv(path)
	return path, nil
}

func validateMountOptions(mountOptions []string) error {
	var Empty struct{}
	// Add invalid options for ImageMount() here.
	invalidOptions := map[string]struct{}{
		"rw": Empty,
	}

	for _, opt := range mountOptions {
		if _, ok := invalidOptions[opt]; ok {
			return fmt.Errorf(" %q option not supported", opt)
		}
	}
	return nil
}
