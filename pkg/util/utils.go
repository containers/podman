package util

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
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
	"github.com/containers/common/pkg/util"
	"github.com/containers/image/v5/types"
	encconfig "github.com/containers/ocicrypt/config"
	enchelpers "github.com/containers/ocicrypt/helpers"
	"github.com/containers/podman/v4/pkg/errorhandling"
	"github.com/containers/podman/v4/pkg/namespaces"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/signal"
	"github.com/containers/storage/pkg/idtools"
	stypes "github.com/containers/storage/types"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"golang.org/x/term"
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
		termPassword, err := term.ReadPassword(0)
		if err != nil {
			return nil, fmt.Errorf("could not read password from terminal: %w", err)
		}
		password = string(termPassword)
	}

	return &types.DockerAuthConfig{
		Username: username,
		Password: password,
	}, nil
}

// StringInSlice is deprecated, use containers/common/pkg/util/StringInSlice
func StringInSlice(s string, sl []string) bool {
	return util.StringInSlice(s, sl)
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
		return -1, errors.New("valid signals are 1 through 64")
	}
	return sig, nil
}

// GetKeepIDMapping returns the mappings and the user to use when keep-id is used
func GetKeepIDMapping(opts *namespaces.KeepIDUserNsOptions) (*stypes.IDMappingOptions, int, int, error) {
	if !rootless.IsRootless() {
		return nil, -1, -1, errors.New("keep-id is only supported in rootless mode")
	}
	options := stypes.IDMappingOptions{
		HostUIDMapping: false,
		HostGIDMapping: false,
	}
	min := func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}

	uid := rootless.GetRootlessUID()
	gid := rootless.GetRootlessGID()
	if opts.UID != nil {
		uid = int(*opts.UID)
	}
	if opts.GID != nil {
		gid = int(*opts.GID)
	}

	uids, gids, err := rootless.GetConfiguredMappings(true)
	if err != nil {
		return nil, -1, -1, fmt.Errorf("cannot read mappings: %w", err)
	}

	maxUID, maxGID := 0, 0
	for _, u := range uids {
		maxUID += u.Size
	}
	for _, g := range gids {
		maxGID += g.Size
	}

	options.UIDMap, options.GIDMap = nil, nil

	if len(uids) > 0 {
		options.UIDMap = append(options.UIDMap, idtools.IDMap{ContainerID: 0, HostID: 1, Size: min(uid, maxUID)})
	}
	options.UIDMap = append(options.UIDMap, idtools.IDMap{ContainerID: uid, HostID: 0, Size: 1})
	if maxUID > uid {
		options.UIDMap = append(options.UIDMap, idtools.IDMap{ContainerID: uid + 1, HostID: uid + 1, Size: maxUID - uid})
	}

	if len(gids) > 0 {
		options.GIDMap = append(options.GIDMap, idtools.IDMap{ContainerID: 0, HostID: 1, Size: min(gid, maxGID)})
	}
	options.GIDMap = append(options.GIDMap, idtools.IDMap{ContainerID: gid, HostID: 0, Size: 1})
	if maxGID > gid {
		options.GIDMap = append(options.GIDMap, idtools.IDMap{ContainerID: gid + 1, HostID: gid + 1, Size: maxGID - gid})
	}

	return &options, uid, gid, nil
}

// GetNoMapMapping returns the mappings and the user to use when nomap is used
func GetNoMapMapping() (*stypes.IDMappingOptions, int, int, error) {
	if !rootless.IsRootless() {
		return nil, -1, -1, errors.New("nomap is only supported in rootless mode")
	}
	options := stypes.IDMappingOptions{
		HostUIDMapping: false,
		HostGIDMapping: false,
	}
	uids, gids, err := rootless.GetConfiguredMappings(false)
	if err != nil {
		return nil, -1, -1, fmt.Errorf("cannot read mappings: %w", err)
	}
	if len(uids) == 0 || len(gids) == 0 {
		return nil, -1, -1, fmt.Errorf("nomap requires additional UIDs or GIDs defined in /etc/subuid and /etc/subgid to function correctly: %w", err)
	}
	options.UIDMap, options.GIDMap = nil, nil
	uid, gid := 0, 0
	for _, u := range uids {
		options.UIDMap = append(options.UIDMap, idtools.IDMap{ContainerID: uid, HostID: uid + 1, Size: u.Size})
		uid += u.Size
	}
	for _, g := range gids {
		options.GIDMap = append(options.GIDMap, idtools.IDMap{ContainerID: gid, HostID: gid + 1, Size: g.Size})
		gid += g.Size
	}
	return &options, 0, 0, nil
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
	if mode.IsKeepID() || mode.IsNoMap() {
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
func ParseInputTime(inputTime string, since bool) (time.Time, error) {
	timeFormats := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02T15:04:05.999999999",
		"2006-01-02Z07:00", "2006-01-02"}
	// iterate the supported time formats
	for _, tf := range timeFormats {
		t, err := time.Parse(tf, inputTime)
		if err == nil {
			return t, nil
		}
	}

	unixTimestamp, err := strconv.ParseFloat(inputTime, 64)
	if err == nil {
		iPart, fPart := math.Modf(unixTimestamp)
		return time.Unix(int64(iPart), int64(fPart*1_000_000_000)).UTC(), nil
	}

	// input might be a duration
	duration, err := time.ParseDuration(inputTime)
	if err != nil {
		return time.Time{}, errors.New("unable to interpret time value")
	}
	if since {
		return time.Now().Add(-duration), nil
	}
	return time.Now().Add(duration), nil
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
			return "", fmt.Errorf("unable to resolve HOME directory: %w", err)
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
			return nil, fmt.Errorf("%s is invalid, sysctl values must be in the form of KEY=VALUE", val)
		}

		trimmed := fmt.Sprintf("%s=%s", strings.TrimSpace(arr[0]), strings.TrimSpace(arr[1]))
		if trimmed != val {
			return nil, fmt.Errorf("'%s' is invalid, extra spaces found", val)
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
			return nil, fmt.Errorf("sysctl '%s' is not allowed", arr[0])
		}
	}
	return sysctl, nil
}

func DefaultContainerConfig() *config.Config {
	return containerConfig
}

func CreateIDFile(path string, id string) error {
	idFile, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating idfile: %w", err)
	}
	defer idFile.Close()
	if _, err = idFile.WriteString(id); err != nil {
		return fmt.Errorf("writing idfile: %w", err)
	}
	return nil
}

// DefaultCPUPeriod is the default CPU period (100ms) in microseconds, which is
// the same default as Kubernetes.
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

func LookupUser(name string) (*user.User, error) {
	// Assume UID lookup first, if it fails look up by username
	if u, err := user.LookupId(name); err == nil {
		return u, nil
	}
	return user.Lookup(name)
}

// SizeOfPath determines the file usage of a given path. it was called volumeSize in v1
// and now is made to be generic and take a path instead of a libpod volume
func SizeOfPath(path string) (uint64, error) {
	var size uint64
	err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			size += uint64(info.Size())
		}
		return err
	})
	return size, err
}

// EncryptConfig translates encryptionKeys into a EncriptionsConfig structure
func EncryptConfig(encryptionKeys []string, encryptLayers []int) (*encconfig.EncryptConfig, *[]int, error) {
	var encLayers *[]int
	var encConfig *encconfig.EncryptConfig

	if len(encryptionKeys) > 0 {
		// encryption
		encLayers = &encryptLayers
		ecc, err := enchelpers.CreateCryptoConfig(encryptionKeys, []string{})
		if err != nil {
			return nil, nil, fmt.Errorf("invalid encryption keys: %w", err)
		}
		cc := encconfig.CombineCryptoConfigs([]encconfig.CryptoConfig{ecc})
		encConfig = cc.EncryptConfig
	}
	return encConfig, encLayers, nil
}

// DecryptConfig translates decryptionKeys into a DescriptionConfig structure
func DecryptConfig(decryptionKeys []string) (*encconfig.DecryptConfig, error) {
	var decryptConfig *encconfig.DecryptConfig
	if len(decryptionKeys) > 0 {
		// decryption
		dcc, err := enchelpers.CreateCryptoConfig([]string{}, decryptionKeys)
		if err != nil {
			return nil, fmt.Errorf("invalid decryption keys: %w", err)
		}
		cc := encconfig.CombineCryptoConfigs([]encconfig.CryptoConfig{dcc})
		decryptConfig = cc.DecryptConfig
	}

	return decryptConfig, nil
}
