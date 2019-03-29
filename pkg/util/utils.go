package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/containers/image/types"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/idtools"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh/terminal"
)

// Helper function to determine the username/password passed
// in the creds string.  It could be either or both.
func parseCreds(creds string) (string, string) {
	if creds == "" {
		return "", ""
	}
	up := strings.SplitN(creds, ":", 2)
	if len(up) == 1 {
		return up[0], ""
	}
	return up[0], up[1]
}

// ParseRegistryCreds takes a credentials string in the form USERNAME:PASSWORD
// and returns a DockerAuthConfig
func ParseRegistryCreds(creds string) (*types.DockerAuthConfig, error) {
	username, password := parseCreds(creds)
	if username == "" {
		fmt.Print("Username: ")
		fmt.Scanln(&username)
	}
	if password == "" {
		fmt.Print("Password: ")
		termPassword, err := terminal.ReadPassword(0)
		if err != nil {
			return nil, errors.Wrapf(err, "could not read password from terminal")
		}
		password = string(termPassword)
	}

	return &types.DockerAuthConfig{
		Username: username,
		Password: password,
	}, nil
}

// StringInSlice determines if a string is in a string slice, returns bool
func StringInSlice(s string, sl []string) bool {
	for _, i := range sl {
		if i == s {
			return true
		}
	}
	return false
}

// GetImageConfig converts the --change flag values in the format "CMD=/bin/bash USER=example"
// to a type v1.ImageConfig
func GetImageConfig(changes []string) (v1.ImageConfig, error) {
	// USER=value | EXPOSE=value | ENV=value | ENTRYPOINT=value |
	// CMD=value | VOLUME=value | WORKDIR=value | LABEL=key=value | STOPSIGNAL=value

	var (
		user       string
		env        []string
		entrypoint []string
		cmd        []string
		workingDir string
		stopSignal string
	)

	exposedPorts := make(map[string]struct{})
	volumes := make(map[string]struct{})
	labels := make(map[string]string)

	for _, ch := range changes {
		pair := strings.Split(ch, "=")
		if len(pair) == 1 {
			return v1.ImageConfig{}, errors.Errorf("no value given for instruction %q", ch)
		}
		switch pair[0] {
		case "USER":
			user = pair[1]
		case "EXPOSE":
			var st struct{}
			exposedPorts[pair[1]] = st
		case "ENV":
			env = append(env, pair[1])
		case "ENTRYPOINT":
			entrypoint = append(entrypoint, pair[1])
		case "CMD":
			cmd = append(cmd, pair[1])
		case "VOLUME":
			var st struct{}
			volumes[pair[1]] = st
		case "WORKDIR":
			workingDir = pair[1]
		case "LABEL":
			if len(pair) == 3 {
				labels[pair[1]] = pair[2]
			} else {
				labels[pair[1]] = ""
			}
		case "STOPSIGNAL":
			stopSignal = pair[1]
		}
	}

	return v1.ImageConfig{
		User:         user,
		ExposedPorts: exposedPorts,
		Env:          env,
		Entrypoint:   entrypoint,
		Cmd:          cmd,
		Volumes:      volumes,
		WorkingDir:   workingDir,
		Labels:       labels,
		StopSignal:   stopSignal,
	}, nil
}

// ParseIDMapping takes idmappings and subuid and subgid maps and returns a storage mapping
func ParseIDMapping(UIDMapSlice, GIDMapSlice []string, subUIDMap, subGIDMap string) (*storage.IDMappingOptions, error) {
	options := storage.IDMappingOptions{
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
			return nil, err
		}
		options.UIDMap = mappings.UIDs()
		options.GIDMap = mappings.GIDs()
	}
	parsedUIDMap, err := idtools.ParseIDMap(UIDMapSlice, "UID")
	if err != nil {
		return nil, err
	}
	parsedGIDMap, err := idtools.ParseIDMap(GIDMapSlice, "GID")
	if err != nil {
		return nil, err
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

var (
	rootlessRuntimeDirOnce sync.Once
	rootlessRuntimeDir     string
)

// GetRootlessRuntimeDir returns the runtime directory when running as non root
func GetRootlessRuntimeDir() (string, error) {
	var rootlessRuntimeDirError error

	rootlessRuntimeDirOnce.Do(func() {
		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
		uid := fmt.Sprintf("%d", rootless.GetRootlessUID())
		if runtimeDir == "" {
			tmpDir := filepath.Join("/run", "user", uid)
			os.MkdirAll(tmpDir, 0700)
			st, err := os.Stat(tmpDir)
			if err == nil && int(st.Sys().(*syscall.Stat_t).Uid) == os.Geteuid() && st.Mode().Perm() == 0700 {
				runtimeDir = tmpDir
			}
		}
		if runtimeDir == "" {
			tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("run-%s", uid))
			os.MkdirAll(tmpDir, 0700)
			st, err := os.Stat(tmpDir)
			if err == nil && int(st.Sys().(*syscall.Stat_t).Uid) == os.Geteuid() && st.Mode().Perm() == 0700 {
				runtimeDir = tmpDir
			}
		}
		if runtimeDir == "" {
			home := os.Getenv("HOME")
			if home == "" {
				rootlessRuntimeDirError = fmt.Errorf("neither XDG_RUNTIME_DIR nor HOME was set non-empty")
				return
			}
			resolvedHome, err := filepath.EvalSymlinks(home)
			if err != nil {
				rootlessRuntimeDirError = errors.Wrapf(err, "cannot resolve %s", home)
				return
			}
			runtimeDir = filepath.Join(resolvedHome, "rundir")
		}
		rootlessRuntimeDir = runtimeDir
	})

	if rootlessRuntimeDirError != nil {
		return "", rootlessRuntimeDirError
	}
	return rootlessRuntimeDir, nil
}

// GetRootlessDirInfo returns the parent path of where the storage for containers and
// volumes will be in rootless mode
func GetRootlessDirInfo() (string, string, error) {
	rootlessRuntime, err := GetRootlessRuntimeDir()
	if err != nil {
		return "", "", err
	}

	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home := os.Getenv("HOME")
		if home == "" {
			return "", "", fmt.Errorf("neither XDG_DATA_HOME nor HOME was set non-empty")
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

type tomlOptionsConfig struct {
	MountProgram string `toml:"mount_program"`
}

type tomlConfig struct {
	Storage struct {
		Driver    string                      `toml:"driver"`
		RunRoot   string                      `toml:"runroot"`
		GraphRoot string                      `toml:"graphroot"`
		Options   struct{ tomlOptionsConfig } `toml:"options"`
	} `toml:"storage"`
}

func getTomlStorage(storeOptions *storage.StoreOptions) *tomlConfig {
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

// WriteStorageConfigFile writes the configuration to a file
func WriteStorageConfigFile(storageOpts *storage.StoreOptions, storageConf string) error {
	os.MkdirAll(filepath.Dir(storageConf), 0755)
	file, err := os.OpenFile(storageConf, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return errors.Wrapf(err, "cannot open %s", storageConf)
	}
	tomlConfiguration := getTomlStorage(storageOpts)
	defer file.Close()
	enc := toml.NewEncoder(file)
	if err := enc.Encode(tomlConfiguration); err != nil {
		os.Remove(storageConf)
		return err
	}
	return nil
}

// ParseInputTime takes the users input and to determine if it is valid and
// returns a time format and error.  The input is compared to known time formats
// or a duration which implies no-duration
func ParseInputTime(inputTime string) (time.Time, error) {
	timeFormats := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02T15:04:05.999999999",
		"2006-01-02Z07:00", "2006-01-02"}
	// iterate the supported time formats
	for _, tf := range timeFormats {
		t, err := time.Parse(tf, inputTime)
		if err == nil {
			return t, nil
		}
	}

	// input might be a duration
	duration, err := time.ParseDuration(inputTime)
	if err != nil {
		return time.Time{}, errors.Errorf("unable to interpret time value")
	}
	return time.Now().Add(-duration), nil
}
