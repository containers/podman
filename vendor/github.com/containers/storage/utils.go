package storage

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
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
func GetRootlessRuntimeDir(rootlessUid int) (string, error) {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")

	if runtimeDir != "" {
		return runtimeDir, nil
	}
	tmpDir := fmt.Sprintf("/run/user/%d", rootlessUid)
	st, err := system.Stat(tmpDir)
	if err == nil && int(st.UID()) == os.Getuid() && st.Mode()&0700 == 0700 && st.Mode()&0066 == 0000 {
		return tmpDir, nil
	}
	tmpDir = fmt.Sprintf("%s/%d", os.TempDir(), rootlessUid)
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		logrus.Errorf("failed to create %s: %v", tmpDir, err)
	} else {
		return tmpDir, nil
	}
	home, err := homeDir()
	if err != nil {
		return "", errors.Wrapf(err, "neither XDG_RUNTIME_DIR nor HOME was set non-empty")
	}
	resolvedHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		return "", errors.Wrapf(err, "cannot resolve %s", home)
	}
	return filepath.Join(resolvedHome, "rundir"), nil
}

// getRootlessDirInfo returns the parent path of where the storage for containers and
// volumes will be in rootless mode
func getRootlessDirInfo(rootlessUid int) (string, string, error) {
	rootlessRuntime, err := GetRootlessRuntimeDir(rootlessUid)
	if err != nil {
		return "", "", err
	}

	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, err := homeDir()
		if err != nil {
			return "", "", errors.Wrapf(err, "neither XDG_DATA_HOME nor HOME was set non-empty")
		}
		// runc doesn't like symlinks in the rootfs path, and at least
		// on CoreOS /home is a symlink to /var/home, so resolve any symlink.
		resolvedHome, err := filepath.EvalSymlinks(home)
		if err != nil {
			return "", "", errors.Wrapf(err, "cannot resolve %s", home)
		}
		dataDir = filepath.Join(resolvedHome, ".local", "share")
	}
	return dataDir, rootlessRuntime, nil
}

// getRootlessStorageOpts returns the storage opts for containers running as non root
func getRootlessStorageOpts(rootlessUid int) (StoreOptions, error) {
	var opts StoreOptions

	dataDir, rootlessRuntime, err := getRootlessDirInfo(rootlessUid)
	if err != nil {
		return opts, err
	}
	opts.RunRoot = rootlessRuntime
	opts.GraphRoot = filepath.Join(dataDir, "containers", "storage")
	if path, err := exec.LookPath("fuse-overlayfs"); err == nil {
		opts.GraphDriverName = "overlay"
		opts.GraphDriverOptions = []string{fmt.Sprintf("overlay.mount_program=%s", path)}
	} else {
		opts.GraphDriverName = "vfs"
	}
	return opts, nil
}

type tomlOptionsConfig struct {
	MountProgram string `toml:"mount_program"`
}

func getTomlStorage(storeOptions *StoreOptions) *tomlConfig {
	config := new(tomlConfig)

	config.Storage.Driver = storeOptions.GraphDriverName
	config.Storage.RunRoot = storeOptions.RunRoot
	config.Storage.GraphRoot = storeOptions.GraphRoot
	for _, i := range storeOptions.GraphDriverOptions {
		s := strings.Split(i, "=")
		if s[0] == "overlay.mount_program" {
			config.Storage.Options.MountProgram = s[1]
		}
	}

	return config
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

// DefaultStoreOptions returns the default storage ops for containers
func DefaultStoreOptions(rootless bool, rootlessUid int) (StoreOptions, error) {
	var (
		defaultRootlessRunRoot   string
		defaultRootlessGraphRoot string
		err                      error
	)
	storageOpts := defaultStoreOptions
	if rootless && rootlessUid != 0 {
		storageOpts, err = getRootlessStorageOpts(rootlessUid)
		if err != nil {
			return storageOpts, err
		}
	}

	storageConf, err := DefaultConfigFile(rootless && rootlessUid != 0)
	if err != nil {
		return storageOpts, err
	}
	_, err = os.Stat(storageConf)
	if err != nil && !os.IsNotExist(err) {
		return storageOpts, errors.Wrapf(err, "cannot stat %s", storageConf)
	}
	if err == nil {
		defaultRootlessRunRoot = storageOpts.RunRoot
		defaultRootlessGraphRoot = storageOpts.GraphRoot
		storageOpts = StoreOptions{}
		ReloadConfigurationFile(storageConf, &storageOpts)
	}

	if rootless && rootlessUid != 0 {
		if err == nil {
			// If the file did not specify a graphroot or runroot,
			// set sane defaults so we don't try and use root-owned
			// directories
			if storageOpts.RunRoot == "" {
				storageOpts.RunRoot = defaultRootlessRunRoot
			}
			if storageOpts.GraphRoot == "" {
				storageOpts.GraphRoot = defaultRootlessGraphRoot
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(storageConf), 0755); err != nil {
				return storageOpts, errors.Wrapf(err, "cannot make directory %s", filepath.Dir(storageConf))
			}
			file, err := os.OpenFile(storageConf, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
			if err != nil {
				return storageOpts, errors.Wrapf(err, "cannot open %s", storageConf)
			}

			tomlConfiguration := getTomlStorage(&storageOpts)
			defer file.Close()
			enc := toml.NewEncoder(file)
			if err := enc.Encode(tomlConfiguration); err != nil {
				os.Remove(storageConf)

				return storageOpts, errors.Wrapf(err, "failed to encode %s", storageConf)
			}
		}
	}
	return storageOpts, nil
}

func homeDir() (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		usr, err := user.Current()
		if err != nil {
			return "", errors.Wrapf(err, "neither XDG_RUNTIME_DIR nor HOME was set non-empty")
		}
		home = usr.HomeDir
	}
	return home, nil
}
