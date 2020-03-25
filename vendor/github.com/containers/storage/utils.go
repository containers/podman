package storage

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
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
		return "", errors.Wrapf(err, "unable to make rootless runtime dir %s", path)
	}
	return path, nil
}

func getRootlessRuntimeDir(rootlessUID int) (string, error) {
	runtimeDir, err := homedir.GetRuntimeDir()
	if err == nil {
		return runtimeDir, nil
	}
	tmpDir := fmt.Sprintf("/run/user/%d", rootlessUID)
	st, err := system.Stat(tmpDir)
	if err == nil && int(st.UID()) == os.Getuid() && st.Mode()&0700 == 0700 && st.Mode()&0066 == 0000 {
		return tmpDir, nil
	}
	tmpDir = fmt.Sprintf("%s/%d", os.TempDir(), rootlessUID)
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		logrus.Errorf("failed to create %s: %v", tmpDir, err)
	} else {
		return tmpDir, nil
	}
	home := homedir.Get()
	if home == "" {
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
		return "", "", errors.Wrapf(err, "cannot resolve %s", home)
	}
	dataDir = filepath.Join(resolvedHome, ".local", "share")

	return dataDir, rootlessRuntime, nil
}

// getRootlessStorageOpts returns the storage opts for containers running as non root
func getRootlessStorageOpts(rootlessUID int) (StoreOptions, error) {
	var opts StoreOptions

	dataDir, rootlessRuntime, err := getRootlessDirInfo(rootlessUID)
	if err != nil {
		return opts, err
	}
	opts.RunRoot = rootlessRuntime
	opts.GraphRoot = filepath.Join(dataDir, "containers", "storage")
	opts.RootlessStoragePath = opts.GraphRoot
	if path, err := exec.LookPath("fuse-overlayfs"); err == nil {
		opts.GraphDriverName = "overlay"
		opts.GraphDriverOptions = []string{fmt.Sprintf("overlay.mount_program=%s", path)}
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

// DefaultStoreOptions returns the default storage ops for containers
func DefaultStoreOptions(rootless bool, rootlessUID int) (StoreOptions, error) {
	var (
		defaultRootlessRunRoot   string
		defaultRootlessGraphRoot string
		err                      error
	)
	storageOpts := defaultStoreOptions
	if rootless && rootlessUID != 0 {
		storageOpts, err = getRootlessStorageOpts(rootlessUID)
		if err != nil {
			return storageOpts, err
		}
	}

	storageConf, err := DefaultConfigFile(rootless && rootlessUID != 0)
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
		reloadConfigurationFileIfNeeded(storageConf, &storageOpts)
	}

	if rootless && rootlessUID != 0 {
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
			if storageOpts.RootlessStoragePath != "" {
				if err = validRootlessStoragePathFormat(storageOpts.RootlessStoragePath); err != nil {
					return storageOpts, err
				}
				rootlessStoragePath := strings.Replace(storageOpts.RootlessStoragePath, "$HOME", homedir.Get(), -1)
				rootlessStoragePath = strings.Replace(rootlessStoragePath, "$UID", strconv.Itoa(rootlessUID), -1)
				usr, err := user.LookupId(strconv.Itoa(rootlessUID))
				if err != nil {
					return storageOpts, err
				}
				rootlessStoragePath = strings.Replace(rootlessStoragePath, "$USER", usr.Username, -1)
				storageOpts.GraphRoot = rootlessStoragePath
			}
		}
	}
	return storageOpts, nil
}

// validRootlessStoragePathFormat checks if the environments contained in the path are accepted
func validRootlessStoragePathFormat(path string) error {
	if !strings.Contains(path, "$") {
		return nil
	}

	splitPaths := strings.SplitAfter(path, "$")
	validEnv := regexp.MustCompile(`^(HOME|USER|UID)([^a-zA-Z]|$)`).MatchString
	if len(splitPaths) > 1 {
		for _, p := range splitPaths[1:] {
			if !validEnv(p) {
				return errors.Errorf("Unrecognized environment variable")
			}
		}
	}
	return nil
}
