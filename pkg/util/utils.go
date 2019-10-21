package util

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/errorhandling"
	"github.com/containers/libpod/pkg/namespaces"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/idtools"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
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

// ParseChanges returns key, value(s) pair for given option.
func ParseChanges(option string) (key string, vals []string, err error) {
	// Supported format as below
	// 1. key=value
	// 2. key value
	// 3. key ["value","value1"]
	if strings.Contains(option, " ") {
		// This handles 2 & 3 conditions.
		var val string
		tokens := strings.SplitAfterN(option, " ", 2)
		if len(tokens) < 2 {
			return "", []string{}, fmt.Errorf("invalid key value %s", option)
		}
		key = strings.Trim(tokens[0], " ") // Need to trim whitespace part of delimeter.
		val = tokens[1]
		if strings.Contains(tokens[1], "[") && strings.Contains(tokens[1], "]") {
			//Trim '[',']' if exist.
			val = strings.TrimLeft(strings.TrimRight(tokens[1], "]"), "[")
		}
		vals = strings.Split(val, ",")
	} else if strings.Contains(option, "=") {
		// handles condition 1.
		tokens := strings.Split(option, "=")
		key = tokens[0]
		vals = tokens[1:]
	} else {
		// either ` ` or `=` must be provided after command
		return "", []string{}, fmt.Errorf("invalid format %s", option)
	}

	if len(vals) == 0 {
		return "", []string{}, errors.Errorf("no value given for instruction %q", key)
	}

	for _, v := range vals {
		//each option must not have ' '., `[`` or `]` & empty strings
		whitespaces := regexp.MustCompile(`[\[\s\]]`)
		if whitespaces.MatchString(v) || len(v) == 0 {
			return "", []string{}, fmt.Errorf("invalid value %s", v)
		}
	}
	return key, vals, nil
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
		key, vals, err := ParseChanges(ch)
		if err != nil {
			return v1.ImageConfig{}, err
		}

		switch key {
		case "USER":
			user = vals[0]
		case "EXPOSE":
			var st struct{}
			exposedPorts[vals[0]] = st
		case "ENV":
			if len(vals) < 2 {
				return v1.ImageConfig{}, errors.Errorf("no value given for environment variable %q", vals[0])
			}
			env = append(env, strings.Join(vals[0:], "="))
		case "ENTRYPOINT":
			// ENTRYPOINT and CMD can have array of strings
			entrypoint = append(entrypoint, vals...)
		case "CMD":
			// ENTRYPOINT and CMD can have array of strings
			cmd = append(cmd, vals...)
		case "VOLUME":
			var st struct{}
			volumes[vals[0]] = st
		case "WORKDIR":
			workingDir = vals[0]
		case "LABEL":
			if len(vals) == 2 {
				labels[vals[0]] = vals[1]
			} else {
				labels[vals[0]] = ""
			}
		case "STOPSIGNAL":
			stopSignal = vals[0]
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
func ParseIDMapping(mode namespaces.UsernsMode, UIDMapSlice, GIDMapSlice []string, subUIDMap, subGIDMap string) (*storage.IDMappingOptions, error) {
	options := storage.IDMappingOptions{
		HostUIDMapping: true,
		HostGIDMapping: true,
	}

	if mode.IsKeepID() {
		if len(UIDMapSlice) > 0 || len(GIDMapSlice) > 0 {
			return nil, errors.New("cannot specify custom mappings with --userns=keep-id")
		}
		if len(subUIDMap) > 0 || len(subGIDMap) > 0 {
			return nil, errors.New("cannot specify subuidmap or subgidmap with --userns=keep-id")
		}
		if rootless.IsRootless() {
			uid := rootless.GetRootlessUID()
			gid := rootless.GetRootlessGID()

			uids, gids, err := rootless.GetConfiguredMappings()
			if err != nil {
				return nil, errors.Wrapf(err, "cannot read mappings")
			}
			maxUID, maxGID := 0, 0
			for _, u := range uids {
				maxUID += u.Size
			}
			for _, g := range gids {
				maxGID += g.Size
			}

			options.UIDMap, options.GIDMap = nil, nil

			options.UIDMap = append(options.UIDMap, idtools.IDMap{ContainerID: 0, HostID: 1, Size: uid})
			options.UIDMap = append(options.UIDMap, idtools.IDMap{ContainerID: uid, HostID: 0, Size: 1})
			options.UIDMap = append(options.UIDMap, idtools.IDMap{ContainerID: uid + 1, HostID: uid + 1, Size: maxUID - uid})

			options.GIDMap = append(options.GIDMap, idtools.IDMap{ContainerID: 0, HostID: 1, Size: gid})
			options.GIDMap = append(options.GIDMap, idtools.IDMap{ContainerID: gid, HostID: 0, Size: 1})
			options.GIDMap = append(options.GIDMap, idtools.IDMap{ContainerID: gid + 1, HostID: gid + 1, Size: maxGID - gid})

			options.HostUIDMapping = false
			options.HostGIDMapping = false
		}
		// Simply ignore the setting and do not setup an inner namespace for root as it is a no-op
		return &options, nil
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
	rootlessConfigHomeDirOnce sync.Once
	rootlessConfigHomeDir     string
	rootlessRuntimeDirOnce    sync.Once
	rootlessRuntimeDir        string
)

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
	if err := os.MkdirAll(filepath.Dir(storageConf), 0755); err != nil {
		return err
	}
	storageFile, err := os.OpenFile(storageConf, os.O_RDWR|os.O_TRUNC, 0600)
	if err != nil {
		return errors.Wrapf(err, "cannot open %s", storageConf)
	}
	tomlConfiguration := getTomlStorage(storageOpts)
	defer errorhandling.CloseQuiet(storageFile)
	enc := toml.NewEncoder(storageFile)
	if err := enc.Encode(tomlConfiguration); err != nil {
		if err := os.Remove(storageConf); err != nil {
			logrus.Errorf("unable to remove file %s", storageConf)
		}
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

// GetGlobalOpts checks all global flags and generates the command string
func GetGlobalOpts(c *cliconfig.RunlabelValues) string {
	globalFlags := map[string]bool{
		"cgroup-manager": true, "cni-config-dir": true, "conmon": true, "default-mounts-file": true,
		"hooks-dir": true, "namespace": true, "root": true, "runroot": true,
		"runtime": true, "storage-driver": true, "storage-opt": true, "syslog": true,
		"trace": true, "network-cmd-path": true, "config": true, "cpu-profile": true,
		"log-level": true, "tmpdir": true}
	const stringSliceType string = "stringSlice"

	var optsCommand []string
	c.PodmanCommand.Command.Flags().VisitAll(func(f *pflag.Flag) {
		if !f.Changed {
			return
		}
		if _, exist := globalFlags[f.Name]; exist {
			if f.Value.Type() == stringSliceType {
				flagValue := strings.TrimSuffix(strings.TrimPrefix(f.Value.String(), "["), "]")
				for _, value := range strings.Split(flagValue, ",") {
					optsCommand = append(optsCommand, fmt.Sprintf("--%s %s", f.Name, value))
				}
			} else {
				optsCommand = append(optsCommand, fmt.Sprintf("--%s %s", f.Name, f.Value.String()))
			}
		}
	})
	return strings.Join(optsCommand, " ")
}

// OpenExclusiveFile opens a file for writing and ensure it doesn't already exist
func OpenExclusiveFile(path string) (*os.File, error) {
	baseDir := filepath.Dir(path)
	if baseDir != "" {
		if _, err := os.Stat(baseDir); err != nil {
			return nil, err
		}
	}
	return os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
}

// PullType whether to pull new image
type PullType int

const (
	// PullImageAlways always try to pull new image when create or run
	PullImageAlways PullType = iota
	// PullImageMissing pulls image if it is not locally
	PullImageMissing
	// PullImageNever will never pull new image
	PullImageNever
)

// ValidatePullType check if the pullType from CLI is valid and returns the valid enum type
// if the value from CLI is invalid returns the error
func ValidatePullType(pullType string) (PullType, error) {
	switch pullType {
	case "always":
		return PullImageAlways, nil
	case "missing":
		return PullImageMissing, nil
	case "never":
		return PullImageNever, nil
	case "":
		return PullImageMissing, nil
	default:
		return PullImageMissing, errors.Errorf("invalid pull type %q", pullType)
	}
}

// ExitCode reads the error message when failing to executing container process
// and then returns 0 if no error, 126 if command does not exist, or 127 for
// all other errors
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	e := strings.ToLower(err.Error())
	if strings.Contains(e, "file not found") ||
		strings.Contains(e, "no such file or directory") {
		return 127
	}

	return 126
}

// HomeDir returns the home directory for the current user.
func HomeDir() (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		usr, err := user.LookupId(fmt.Sprintf("%d", rootless.GetRootlessUID()))
		if err != nil {
			return "", errors.Wrapf(err, "unable to resolve HOME directory")
		}
		home = usr.HomeDir
	}
	return home, nil
}
