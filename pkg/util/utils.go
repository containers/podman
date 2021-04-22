package util

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/containers/common/pkg/config"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v3/pkg/errorhandling"
	"github.com/containers/podman/v3/pkg/namespaces"
	"github.com/containers/podman/v3/pkg/rootless"
	"github.com/containers/podman/v3/pkg/signal"
	"github.com/containers/storage/pkg/idtools"
	stypes "github.com/containers/storage/types"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

var containerConfig *config.Config

func init() {
	var err error
	containerConfig, err = config.Default()
	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
}

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

// StringMatchRegexSlice determines if a given string matches one of the given regexes, returns bool
func StringMatchRegexSlice(s string, re []string) bool {
	for _, r := range re {
		m, err := regexp.MatchString(r, s)
		if err == nil && m {
			return true
		}
	}
	return false
}

// ImageConfig is a wrapper around the OCIv1 Image Configuration struct exported
// by containers/image, but containing additional fields that are not supported
// by OCIv1 (but are by Docker v2) - notably OnBuild.
type ImageConfig struct {
	v1.ImageConfig
	OnBuild []string
}

// GetImageConfig produces a v1.ImageConfig from the --change flag that is
// accepted by several Podman commands. It accepts a (limited subset) of
// Dockerfile instructions.
func GetImageConfig(changes []string) (ImageConfig, error) {
	// Valid changes:
	// USER
	// EXPOSE
	// ENV
	// ENTRYPOINT
	// CMD
	// VOLUME
	// WORKDIR
	// LABEL
	// STOPSIGNAL
	// ONBUILD

	config := ImageConfig{}

	for _, change := range changes {
		// First, let's assume proper Dockerfile format - space
		// separator between instruction and value
		split := strings.SplitN(change, " ", 2)

		if len(split) != 2 {
			split = strings.SplitN(change, "=", 2)
			if len(split) != 2 {
				return ImageConfig{}, errors.Errorf("invalid change %q - must be formatted as KEY VALUE", change)
			}
		}

		outerKey := strings.ToUpper(strings.TrimSpace(split[0]))
		value := strings.TrimSpace(split[1])
		switch outerKey {
		case "USER":
			// Assume literal contents are the user.
			if value == "" {
				return ImageConfig{}, errors.Errorf("invalid change %q - must provide a value to USER", change)
			}
			config.User = value
		case "EXPOSE":
			// EXPOSE is either [portnum] or
			// [portnum]/[proto]
			// Protocol must be "tcp" or "udp"
			splitPort := strings.Split(value, "/")
			if len(splitPort) > 2 {
				return ImageConfig{}, errors.Errorf("invalid change %q - EXPOSE port must be formatted as PORT[/PROTO]", change)
			}
			portNum, err := strconv.Atoi(splitPort[0])
			if err != nil {
				return ImageConfig{}, errors.Wrapf(err, "invalid change %q - EXPOSE port must be an integer", change)
			}
			if portNum > 65535 || portNum <= 0 {
				return ImageConfig{}, errors.Errorf("invalid change %q - EXPOSE port must be a valid port number", change)
			}
			proto := "tcp"
			if len(splitPort) > 1 {
				testProto := strings.ToLower(splitPort[1])
				switch testProto {
				case "tcp", "udp":
					proto = testProto
				default:
					return ImageConfig{}, errors.Errorf("invalid change %q - EXPOSE protocol must be TCP or UDP", change)
				}
			}
			if config.ExposedPorts == nil {
				config.ExposedPorts = make(map[string]struct{})
			}
			config.ExposedPorts[fmt.Sprintf("%d/%s", portNum, proto)] = struct{}{}
		case "ENV":
			// Format is either:
			// ENV key=value
			// ENV key=value key=value ...
			// ENV key value
			// Both keys and values can be surrounded by quotes to group them.
			// For now: we only support key=value
			// We will attempt to strip quotation marks if present.

			var (
				key, val string
			)

			splitEnv := strings.SplitN(value, "=", 2)
			key = splitEnv[0]
			// We do need a key
			if key == "" {
				return ImageConfig{}, errors.Errorf("invalid change %q - ENV must have at least one argument", change)
			}
			// Perfectly valid to not have a value
			if len(splitEnv) == 2 {
				val = splitEnv[1]
			}

			if strings.HasPrefix(key, `"`) && strings.HasSuffix(key, `"`) {
				key = strings.TrimPrefix(strings.TrimSuffix(key, `"`), `"`)
			}
			if strings.HasPrefix(val, `"`) && strings.HasSuffix(val, `"`) {
				val = strings.TrimPrefix(strings.TrimSuffix(val, `"`), `"`)
			}
			config.Env = append(config.Env, fmt.Sprintf("%s=%s", key, val))
		case "ENTRYPOINT":
			// Two valid forms.
			// First, JSON array.
			// Second, not a JSON array - we interpret this as an
			// argument to `sh -c`, unless empty, in which case we
			// just use a blank entrypoint.
			testUnmarshal := []string{}
			if err := json.Unmarshal([]byte(value), &testUnmarshal); err != nil {
				// It ain't valid JSON, so assume it's an
				// argument to sh -c if not empty.
				if value != "" {
					config.Entrypoint = []string{"/bin/sh", "-c", value}
				} else {
					config.Entrypoint = []string{}
				}
			} else {
				// Valid JSON
				config.Entrypoint = testUnmarshal
			}
		case "CMD":
			// Same valid forms as entrypoint.
			// However, where ENTRYPOINT assumes that 'ENTRYPOINT '
			// means no entrypoint, CMD assumes it is 'sh -c' with
			// no third argument.
			testUnmarshal := []string{}
			if err := json.Unmarshal([]byte(value), &testUnmarshal); err != nil {
				// It ain't valid JSON, so assume it's an
				// argument to sh -c.
				// Only include volume if it's not ""
				config.Cmd = []string{"/bin/sh", "-c"}
				if value != "" {
					config.Cmd = append(config.Cmd, value)
				}
			} else {
				// Valid JSON
				config.Cmd = testUnmarshal
			}
		case "VOLUME":
			// Either a JSON array or a set of space-separated
			// paths.
			// Acts rather similar to ENTRYPOINT and CMD, but always
			// appends rather than replacing, and no sh -c prepend.
			testUnmarshal := []string{}
			if err := json.Unmarshal([]byte(value), &testUnmarshal); err != nil {
				// Not valid JSON, so split on spaces
				testUnmarshal = strings.Split(value, " ")
			}
			if len(testUnmarshal) == 0 {
				return ImageConfig{}, errors.Errorf("invalid change %q - must provide at least one argument to VOLUME", change)
			}
			for _, vol := range testUnmarshal {
				if vol == "" {
					return ImageConfig{}, errors.Errorf("invalid change %q - VOLUME paths must not be empty", change)
				}
				if config.Volumes == nil {
					config.Volumes = make(map[string]struct{})
				}
				config.Volumes[vol] = struct{}{}
			}
		case "WORKDIR":
			// This can be passed multiple times.
			// Each successive invocation is treated as relative to
			// the previous one - so WORKDIR /A, WORKDIR b,
			// WORKDIR c results in /A/b/c
			// Just need to check it's not empty...
			if value == "" {
				return ImageConfig{}, errors.Errorf("invalid change %q - must provide a non-empty WORKDIR", change)
			}
			config.WorkingDir = filepath.Join(config.WorkingDir, value)
		case "LABEL":
			// Same general idea as ENV, but we no longer allow " "
			// as a separator.
			// We didn't do that for ENV either, so nice and easy.
			// Potentially problematic: LABEL might theoretically
			// allow an = in the key? If people really do this, we
			// may need to investigate more advanced parsing.
			var (
				key, val string
			)

			splitLabel := strings.SplitN(value, "=", 2)
			// Unlike ENV, LABEL must have a value
			if len(splitLabel) != 2 {
				return ImageConfig{}, errors.Errorf("invalid change %q - LABEL must be formatted key=value", change)
			}
			key = splitLabel[0]
			val = splitLabel[1]

			if strings.HasPrefix(key, `"`) && strings.HasSuffix(key, `"`) {
				key = strings.TrimPrefix(strings.TrimSuffix(key, `"`), `"`)
			}
			if strings.HasPrefix(val, `"`) && strings.HasSuffix(val, `"`) {
				val = strings.TrimPrefix(strings.TrimSuffix(val, `"`), `"`)
			}
			// Check key after we strip quotations
			if key == "" {
				return ImageConfig{}, errors.Errorf("invalid change %q - LABEL must have a non-empty key", change)
			}
			if config.Labels == nil {
				config.Labels = make(map[string]string)
			}
			config.Labels[key] = val
		case "STOPSIGNAL":
			// Check the provided signal for validity.
			killSignal, err := ParseSignal(value)
			if err != nil {
				return ImageConfig{}, errors.Wrapf(err, "invalid change %q - KILLSIGNAL must be given a valid signal", change)
			}
			config.StopSignal = fmt.Sprintf("%d", killSignal)
		case "ONBUILD":
			// Onbuild always appends.
			if value == "" {
				return ImageConfig{}, errors.Errorf("invalid change %q - ONBUILD must be given an argument", change)
			}
			config.OnBuild = append(config.OnBuild, value)
		default:
			return ImageConfig{}, errors.Errorf("invalid change %q - invalid instruction %s", change, outerKey)
		}
	}

	return config, nil
}

// ParseSignal parses and validates a signal name or number.
func ParseSignal(rawSignal string) (syscall.Signal, error) {
	// Strip off leading dash, to allow -1 or -HUP
	basename := strings.TrimPrefix(rawSignal, "-")

	sig, err := signal.ParseSignal(basename)
	if err != nil {
		return -1, err
	}
	// 64 is SIGRTMAX; wish we could get this from a standard Go library
	if sig < 1 || sig > 64 {
		return -1, errors.Errorf("valid signals are 1 through 64")
	}
	return sig, nil
}

// GetKeepIDMapping returns the mappings and the user to use when keep-id is used
func GetKeepIDMapping() (*stypes.IDMappingOptions, int, int, error) {
	options := stypes.IDMappingOptions{
		HostUIDMapping: true,
		HostGIDMapping: true,
	}
	uid, gid := 0, 0
	if rootless.IsRootless() {
		min := func(a, b int) int {
			if a < b {
				return a
			}
			return b
		}

		uid = rootless.GetRootlessUID()
		gid = rootless.GetRootlessGID()

		uids, gids, err := rootless.GetConfiguredMappings()
		if err != nil {
			return nil, -1, -1, errors.Wrapf(err, "cannot read mappings")
		}
		maxUID, maxGID := 0, 0
		for _, u := range uids {
			maxUID += u.Size
		}
		for _, g := range gids {
			maxGID += g.Size
		}

		options.UIDMap, options.GIDMap = nil, nil

		options.UIDMap = append(options.UIDMap, idtools.IDMap{ContainerID: 0, HostID: 1, Size: min(uid, maxUID)})
		options.UIDMap = append(options.UIDMap, idtools.IDMap{ContainerID: uid, HostID: 0, Size: 1})
		if maxUID > uid {
			options.UIDMap = append(options.UIDMap, idtools.IDMap{ContainerID: uid + 1, HostID: uid + 1, Size: maxUID - uid})
		}

		options.GIDMap = append(options.GIDMap, idtools.IDMap{ContainerID: 0, HostID: 1, Size: min(gid, maxGID)})
		options.GIDMap = append(options.GIDMap, idtools.IDMap{ContainerID: gid, HostID: 0, Size: 1})
		if maxGID > gid {
			options.GIDMap = append(options.GIDMap, idtools.IDMap{ContainerID: gid + 1, HostID: gid + 1, Size: maxGID - gid})
		}

		options.HostUIDMapping = false
		options.HostGIDMapping = false
	}
	// Simply ignore the setting and do not setup an inner namespace for root as it is a no-op
	return &options, uid, gid, nil
}

// ParseIDMapping takes idmappings and subuid and subgid maps and returns a storage mapping
func ParseIDMapping(mode namespaces.UsernsMode, uidMapSlice, gidMapSlice []string, subUIDMap, subGIDMap string) (*stypes.IDMappingOptions, error) {
	options := stypes.IDMappingOptions{
		HostUIDMapping: true,
		HostGIDMapping: true,
	}

	if mode.IsAuto() {
		var err error
		options.HostUIDMapping = false
		options.HostGIDMapping = false
		options.AutoUserNs = true
		opts, err := mode.GetAutoOptions()
		if err != nil {
			return nil, err
		}
		options.AutoUserNsOpts = *opts
		return &options, nil
	}
	if mode.IsKeepID() {
		options.HostUIDMapping = false
		options.HostGIDMapping = false
		return &options, nil
	}

	if subGIDMap == "" && subUIDMap != "" {
		subGIDMap = subUIDMap
	}
	if subUIDMap == "" && subGIDMap != "" {
		subUIDMap = subGIDMap
	}
	if len(gidMapSlice) == 0 && len(uidMapSlice) != 0 {
		gidMapSlice = uidMapSlice
	}
	if len(uidMapSlice) == 0 && len(gidMapSlice) != 0 {
		uidMapSlice = gidMapSlice
	}

	if subUIDMap != "" && subGIDMap != "" {
		mappings, err := idtools.NewIDMappings(subUIDMap, subGIDMap)
		if err != nil {
			return nil, err
		}
		options.UIDMap = mappings.UIDs()
		options.GIDMap = mappings.GIDs()
	}
	parsedUIDMap, err := idtools.ParseIDMap(uidMapSlice, "UID")
	if err != nil {
		return nil, err
	}
	parsedGIDMap, err := idtools.ParseIDMap(gidMapSlice, "GID")
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

func getTomlStorage(storeOptions *stypes.StoreOptions) *tomlConfig {
	config := new(tomlConfig)

	config.Storage.Driver = storeOptions.GraphDriverName
	config.Storage.RunRoot = storeOptions.RunRoot
	config.Storage.GraphRoot = storeOptions.GraphRoot
	for _, i := range storeOptions.GraphDriverOptions {
		s := strings.SplitN(i, "=", 2)
		if s[0] == "overlay.mount_program" && len(s) == 2 {
			config.Storage.Options.MountProgram = s[1]
		}
	}

	return config
}

// WriteStorageConfigFile writes the configuration to a file
func WriteStorageConfigFile(storageOpts *stypes.StoreOptions, storageConf string) error {
	if err := os.MkdirAll(filepath.Dir(storageConf), 0755); err != nil {
		return err
	}
	storageFile, err := os.OpenFile(storageConf, os.O_RDWR|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	tomlConfiguration := getTomlStorage(storageOpts)
	defer errorhandling.CloseQuiet(storageFile)
	enc := toml.NewEncoder(storageFile)
	if err := enc.Encode(tomlConfiguration); err != nil {
		if err := os.Remove(storageConf); err != nil {
			logrus.Error(err)
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

	unixTimestamp, err := strconv.ParseInt(inputTime, 10, 64)
	if err == nil {
		return time.Unix(unixTimestamp, 0), nil
	}

	// input might be a duration
	duration, err := time.ParseDuration(inputTime)
	if err != nil {
		return time.Time{}, errors.Errorf("unable to interpret time value")
	}
	return time.Now().Add(-duration), nil
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

func Tmpdir() string {
	tmpdir := os.Getenv("TMPDIR")
	if tmpdir == "" {
		tmpdir = "/var/tmp"
	}

	return tmpdir
}

// ValidateSysctls validates a list of sysctl and returns it.
func ValidateSysctls(strSlice []string) (map[string]string, error) {
	sysctl := make(map[string]string)
	validSysctlMap := map[string]bool{
		"kernel.msgmax":          true,
		"kernel.msgmnb":          true,
		"kernel.msgmni":          true,
		"kernel.sem":             true,
		"kernel.shmall":          true,
		"kernel.shmmax":          true,
		"kernel.shmmni":          true,
		"kernel.shm_rmid_forced": true,
	}
	validSysctlPrefixes := []string{
		"net.",
		"fs.mqueue.",
	}

	for _, val := range strSlice {
		foundMatch := false
		arr := strings.Split(val, "=")
		if len(arr) < 2 {
			return nil, errors.Errorf("%s is invalid, sysctl values must be in the form of KEY=VALUE", val)
		}
		if validSysctlMap[arr[0]] {
			sysctl[arr[0]] = arr[1]
			continue
		}

		for _, prefix := range validSysctlPrefixes {
			if strings.HasPrefix(arr[0], prefix) {
				sysctl[arr[0]] = arr[1]
				foundMatch = true
				break
			}
		}
		if !foundMatch {
			return nil, errors.Errorf("sysctl '%s' is not allowed", arr[0])
		}
	}
	return sysctl, nil
}

func DefaultContainerConfig() *config.Config {
	return containerConfig
}

func CreateCidFile(cidfile string, id string) error {
	cidFile, err := OpenExclusiveFile(cidfile)
	if err != nil {
		if os.IsExist(err) {
			return errors.Errorf("container id file exists. Ensure another container is not using it or delete %s", cidfile)
		}
		return errors.Errorf("error opening cidfile %s", cidfile)
	}
	if _, err = cidFile.WriteString(id); err != nil {
		logrus.Error(err)
	}
	cidFile.Close()
	return nil
}

// DefaultCPUPeriod is the default CPU period is 100us, which is the same default
// as Kubernetes.
const DefaultCPUPeriod uint64 = 100000

// CoresToPeriodAndQuota converts a fraction of cores to the equivalent
// Completely Fair Scheduler (CFS) parameters period and quota.
//
// Cores is a fraction of the CFS period that a container may use. Period and
// Quota are in microseconds.
func CoresToPeriodAndQuota(cores float64) (uint64, int64) {
	return DefaultCPUPeriod, int64(cores * float64(DefaultCPUPeriod))
}

// PeriodAndQuotaToCores takes the CFS parameters period and quota and returns
// a fraction that represents the limit to the number of cores that can be
// utilized over the scheduling period.
//
// Cores is a fraction of the CFS period that a container may use. Period and
// Quota are in microseconds.
func PeriodAndQuotaToCores(period uint64, quota int64) float64 {
	return float64(quota) / float64(period)
}

// IDtoolsToRuntimeSpec converts idtools ID mapping to the one of the runtime spec.
func IDtoolsToRuntimeSpec(idMaps []idtools.IDMap) (convertedIDMap []specs.LinuxIDMapping) {
	for _, idmap := range idMaps {
		tempIDMap := specs.LinuxIDMapping{
			ContainerID: uint32(idmap.ContainerID),
			HostID:      uint32(idmap.HostID),
			Size:        uint32(idmap.Size),
		}
		convertedIDMap = append(convertedIDMap, tempIDMap)
	}
	return convertedIDMap
}

var socketPath string

func SetSocketPath(path string) {
	socketPath = path
}

func SocketPath() (string, error) {
	if socketPath != "" {
		return socketPath, nil
	}
	xdg, err := GetRuntimeDir()
	if err != nil {
		return "", err
	}
	if len(xdg) == 0 {
		// If no xdg is returned, assume root socket
		xdg = "/run"
	}

	// Glue the socket path together
	return filepath.Join(xdg, "podman", "podman.sock"), nil
}
