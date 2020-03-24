package sysregistriesv2

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage/pkg/homedir"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// systemRegistriesConfPath is the path to the system-wide registry
// configuration file and is used to add/subtract potential registries for
// obtaining images.  You can override this at build time with
// -ldflags '-X github.com/containers/image/sysregistries.systemRegistriesConfPath=$your_path'
var systemRegistriesConfPath = builtinRegistriesConfPath

// builtinRegistriesConfPath is the path to the registry configuration file.
// DO NOT change this, instead see systemRegistriesConfPath above.
const builtinRegistriesConfPath = "/etc/containers/registries.conf"

// systemRegistriesConfDirPath is the path to the system-wide registry
// configuration directory and is used to add/subtract potential registries for
// obtaining images.  You can override this at build time with
// -ldflags '-X github.com/containers/image/sysregistries.systemRegistriesConfDirecotyPath=$your_path'
var systemRegistriesConfDirPath = builtinRegistriesConfDirPath

// builtinRegistriesConfDirPath is the path to the registry configuration directory.
// DO NOT change this, instead see systemRegistriesConfDirectoryPath above.
const builtinRegistriesConfDirPath = "/etc/containers/registries.conf.d"

// Endpoint describes a remote location of a registry.
type Endpoint struct {
	// The endpoint's remote location.
	Location string `toml:"location,omitempty"`
	// If true, certs verification will be skipped and HTTP (non-TLS)
	// connections will be allowed.
	Insecure bool `toml:"insecure,omitempty"`
}

// userRegistriesFile is the path to the per user registry configuration file.
var userRegistriesFile = filepath.FromSlash(".config/containers/registries.conf")

// userRegistriesDir is the path to the per user registry configuration file.
var userRegistriesDir = filepath.FromSlash(".config/containers/registries.conf.d")

// rewriteReference will substitute the provided reference `prefix` to the
// endpoints `location` from the `ref` and creates a new named reference from it.
// The function errors if the newly created reference is not parsable.
func (e *Endpoint) rewriteReference(ref reference.Named, prefix string) (reference.Named, error) {
	refString := ref.String()
	if !refMatchesPrefix(refString, prefix) {
		return nil, fmt.Errorf("invalid prefix '%v' for reference '%v'", prefix, refString)
	}

	newNamedRef := strings.Replace(refString, prefix, e.Location, 1)
	newParsedRef, err := reference.ParseNamed(newNamedRef)
	if err != nil {
		return nil, errors.Wrapf(err, "error rewriting reference")
	}

	return newParsedRef, nil
}

// Registry represents a registry.
type Registry struct {
	// Prefix is used for matching images, and to translate one namespace to
	// another.  If `Prefix="example.com/bar"`, `location="example.com/foo/bar"`
	// and we pull from "example.com/bar/myimage:latest", the image will
	// effectively be pulled from "example.com/foo/bar/myimage:latest".
	// If no Prefix is specified, it defaults to the specified location.
	Prefix string `toml:"prefix"`
	// A registry is an Endpoint too
	Endpoint
	// The registry's mirrors.
	Mirrors []Endpoint `toml:"mirror,omitempty"`
	// If true, pulling from the registry will be blocked.
	Blocked bool `toml:"blocked,omitempty"`
	// If true, mirrors will only be used for digest pulls. Pulling images by
	// tag can potentially yield different images, depending on which endpoint
	// we pull from.  Forcing digest-pulls for mirrors avoids that issue.
	MirrorByDigestOnly bool `toml:"mirror-by-digest-only,omitempty"`
}

// PullSource consists of an Endpoint and a Reference. Note that the reference is
// rewritten according to the registries prefix and the Endpoint's location.
type PullSource struct {
	Endpoint  Endpoint
	Reference reference.Named
}

// PullSourcesFromReference returns a slice of PullSource's based on the passed
// reference.
func (r *Registry) PullSourcesFromReference(ref reference.Named) ([]PullSource, error) {
	var endpoints []Endpoint

	if r.MirrorByDigestOnly {
		// Only use mirrors when the reference is a digest one.
		if _, isDigested := ref.(reference.Canonical); isDigested {
			endpoints = append(r.Mirrors, r.Endpoint)
		} else {
			endpoints = []Endpoint{r.Endpoint}
		}
	} else {
		endpoints = append(r.Mirrors, r.Endpoint)
	}

	sources := []PullSource{}
	for _, ep := range endpoints {
		rewritten, err := ep.rewriteReference(ref, r.Prefix)
		if err != nil {
			return nil, err
		}
		sources = append(sources, PullSource{Endpoint: ep, Reference: rewritten})
	}

	return sources, nil
}

// V1TOMLregistries is for backwards compatibility to sysregistries v1
type V1TOMLregistries struct {
	Registries []string `toml:"registries"`
}

// V1TOMLConfig is for backwards compatibility to sysregistries v1
type V1TOMLConfig struct {
	Search   V1TOMLregistries `toml:"search"`
	Insecure V1TOMLregistries `toml:"insecure"`
	Block    V1TOMLregistries `toml:"block"`
}

// V1RegistriesConf is the sysregistries v1 configuration format.
type V1RegistriesConf struct {
	V1TOMLConfig `toml:"registries"`
}

// Nonempty returns true if config contains at least one configuration entry.
func (config *V1RegistriesConf) Nonempty() bool {
	return (len(config.V1TOMLConfig.Search.Registries) != 0 ||
		len(config.V1TOMLConfig.Insecure.Registries) != 0 ||
		len(config.V1TOMLConfig.Block.Registries) != 0)
}

// V2RegistriesConf is the sysregistries v2 configuration format.
type V2RegistriesConf struct {
	Registries []Registry `toml:"registry"`
	// An array of host[:port] (not prefix!) entries to use for resolving unqualified image references
	UnqualifiedSearchRegistries []string `toml:"unqualified-search-registries"`
}

// Nonempty returns true if config contains at least one configuration entry.
func (config *V2RegistriesConf) Nonempty() bool {
	return (len(config.Registries) != 0 ||
		len(config.UnqualifiedSearchRegistries) != 0)
}

// tomlConfig is the data type used to unmarshal the toml config.
type tomlConfig struct {
	V2RegistriesConf
	V1RegistriesConf // for backwards compatibility with sysregistries v1
}

// InvalidRegistries represents an invalid registry configurations.  An example
// is when "registry.com" is defined multiple times in the configuration but
// with conflicting security settings.
type InvalidRegistries struct {
	s string
}

// Error returns the error string.
func (e *InvalidRegistries) Error() string {
	return e.s
}

// parseLocation parses the input string, performs some sanity checks and returns
// the sanitized input string.  An error is returned if the input string is
// empty or if contains an "http{s,}://" prefix.
func parseLocation(input string) (string, error) {
	trimmed := strings.TrimRight(input, "/")

	if trimmed == "" {
		return "", &InvalidRegistries{s: "invalid location: cannot be empty"}
	}

	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		msg := fmt.Sprintf("invalid location '%s': URI schemes are not supported", input)
		return "", &InvalidRegistries{s: msg}
	}

	return trimmed, nil
}

// ConvertToV2 returns a v2 config corresponding to a v1 one.
func (config *V1RegistriesConf) ConvertToV2() (*V2RegistriesConf, error) {
	regMap := make(map[string]*Registry)
	// The order of the registries is not really important, but make it deterministic (the same for the same config file)
	// to minimize behavior inconsistency and not contribute to difficult-to-reproduce situations.
	registryOrder := []string{}

	getRegistry := func(location string) (*Registry, error) { // Note: _pointer_ to a long-lived object
		var err error
		location, err = parseLocation(location)
		if err != nil {
			return nil, err
		}
		reg, exists := regMap[location]
		if !exists {
			reg = &Registry{
				Endpoint: Endpoint{Location: location},
				Mirrors:  []Endpoint{},
				Prefix:   location,
			}
			regMap[location] = reg
			registryOrder = append(registryOrder, location)
		}
		return reg, nil
	}

	for _, blocked := range config.V1TOMLConfig.Block.Registries {
		reg, err := getRegistry(blocked)
		if err != nil {
			return nil, err
		}
		reg.Blocked = true
	}
	for _, insecure := range config.V1TOMLConfig.Insecure.Registries {
		reg, err := getRegistry(insecure)
		if err != nil {
			return nil, err
		}
		reg.Insecure = true
	}

	res := &V2RegistriesConf{
		UnqualifiedSearchRegistries: config.V1TOMLConfig.Search.Registries,
	}
	for _, location := range registryOrder {
		reg := regMap[location]
		res.Registries = append(res.Registries, *reg)
	}
	return res, nil
}

// anchoredDomainRegexp is an internal implementation detail of postProcess, defining the valid values of elements of UnqualifiedSearchRegistries.
var anchoredDomainRegexp = regexp.MustCompile("^" + reference.DomainRegexp.String() + "$")

// postProcess checks the consistency of all the configuration, looks for conflicts,
// and normalizes the configuration (e.g., sets the Prefix to Location if not set).
func (config *V2RegistriesConf) postProcess() error {
	regMap := make(map[string][]*Registry)

	for i := range config.Registries {
		reg := &config.Registries[i]
		// make sure Location and Prefix are valid
		var err error
		reg.Location, err = parseLocation(reg.Location)
		if err != nil {
			return err
		}

		if reg.Prefix == "" {
			reg.Prefix = reg.Location
		} else {
			reg.Prefix, err = parseLocation(reg.Prefix)
			if err != nil {
				return err
			}
		}

		// make sure mirrors are valid
		for _, mir := range reg.Mirrors {
			mir.Location, err = parseLocation(mir.Location)
			if err != nil {
				return err
			}
		}
		regMap[reg.Location] = append(regMap[reg.Location], reg)
	}

	// Given a registry can be mentioned multiple times (e.g., to have
	// multiple prefixes backed by different mirrors), we need to make sure
	// there are no conflicts among them.
	//
	// Note: we need to iterate over the registries array to ensure a
	// deterministic behavior which is not guaranteed by maps.
	for _, reg := range config.Registries {
		others, ok := regMap[reg.Location]
		if !ok {
			return fmt.Errorf("Internal error in V2RegistriesConf.PostProcess: entry in regMap is missing")
		}
		for _, other := range others {
			if reg.Insecure != other.Insecure {
				msg := fmt.Sprintf("registry '%s' is defined multiple times with conflicting 'insecure' setting", reg.Location)
				return &InvalidRegistries{s: msg}
			}
			if reg.Blocked != other.Blocked {
				msg := fmt.Sprintf("registry '%s' is defined multiple times with conflicting 'blocked' setting", reg.Location)
				return &InvalidRegistries{s: msg}
			}
		}
	}

	for i := range config.UnqualifiedSearchRegistries {
		registry, err := parseLocation(config.UnqualifiedSearchRegistries[i])
		if err != nil {
			return err
		}
		if !anchoredDomainRegexp.MatchString(registry) {
			return &InvalidRegistries{fmt.Sprintf("Invalid unqualified-search-registries entry %#v", registry)}
		}
		config.UnqualifiedSearchRegistries[i] = registry
	}

	// Registries are ordered and the first longest prefix always wins,
	// rendering later items with the same prefix non-existent. We cannot error
	// out anymore as this might break existing users, so let's just ignore them
	// to guarantee that the same prefix exists only once.
	knownPrefixes := make(map[string]bool)
	uniqueRegistries := []Registry{}
	for i := range config.Registries {
		// TODO: should we warn if we see the same prefix being used multiple times?
		if _, exists := knownPrefixes[config.Registries[i].Prefix]; !exists {
			knownPrefixes[config.Registries[i].Prefix] = true
			uniqueRegistries = append(uniqueRegistries, config.Registries[i])
		}
	}
	config.Registries = uniqueRegistries

	return nil
}

// ConfigPath returns the path to the system-wide registry configuration file.
func ConfigPath(ctx *types.SystemContext) string {
	if ctx != nil && ctx.SystemRegistriesConfPath != "" {
		return ctx.SystemRegistriesConfPath
	}

	userRegistriesFilePath := filepath.Join(homedir.Get(), userRegistriesFile)
	if _, err := os.Stat(userRegistriesFilePath); err == nil {
		return userRegistriesFilePath
	}

	if ctx != nil && ctx.RootForImplicitAbsolutePaths != "" {
		return filepath.Join(ctx.RootForImplicitAbsolutePaths, systemRegistriesConfPath)
	}

	return systemRegistriesConfPath
}

// ConfigDirPath returns the path to the system-wide directory for drop-in
// registry configuration files.
func ConfigDirPath(ctx *types.SystemContext) string {
	if ctx != nil && ctx.SystemRegistriesConfDirPath != "" {
		return ctx.SystemRegistriesConfDirPath
	}

	userRegistriesDirPath := filepath.Join(homedir.Get(), userRegistriesDir)
	if _, err := os.Stat(userRegistriesDirPath); err == nil {
		return userRegistriesDirPath
	}

	if ctx != nil && ctx.RootForImplicitAbsolutePaths != "" {
		return filepath.Join(ctx.RootForImplicitAbsolutePaths, systemRegistriesConfDirPath)
	}

	return systemRegistriesConfDirPath
}

// configWrapper is used to store the paths from ConfigPath and ConfigDirPath
// and acts as a key to the internal cache.
type configWrapper struct {
	configPath    string
	configDirPath string
}

// newConfigWrapper returns a configWrapper for the specified SystemContext.
func newConfigWrapper(ctx *types.SystemContext) configWrapper {
	return configWrapper{
		configPath:    ConfigPath(ctx),
		configDirPath: ConfigDirPath(ctx),
	}
}

// configMutex is used to synchronize concurrent accesses to configCache.
var configMutex = sync.Mutex{}

// configCache caches already loaded configs with config paths as keys and is
// used to avoid redundantly parsing configs. Concurrent accesses to the cache
// are synchronized via configMutex.
var configCache = make(map[configWrapper]*V2RegistriesConf)

// InvalidateCache invalidates the registry cache.  This function is meant to be
// used for long-running processes that need to reload potential changes made to
// the cached registry config files.
func InvalidateCache() {
	configMutex.Lock()
	defer configMutex.Unlock()
	configCache = make(map[configWrapper]*V2RegistriesConf)
}

// getConfig returns the config object corresponding to ctx, loading it if it is not yet cached.
func getConfig(ctx *types.SystemContext) (*V2RegistriesConf, error) {
	wrapper := newConfigWrapper(ctx)
	configMutex.Lock()
	if config, inCache := configCache[wrapper]; inCache {
		configMutex.Unlock()
		return config, nil
	}
	configMutex.Unlock()

	return tryUpdatingCache(ctx, wrapper)
}

// dropInConfigs returns a slice of drop-in-configs from the registries.conf.d
// directory.
func dropInConfigs(wrapper configWrapper) ([]string, error) {
	var configs []string

	err := filepath.Walk(wrapper.configDirPath,
		// WalkFunc to read additional configs
		func(path string, info os.FileInfo, err error) error {
			switch {
			case err != nil:
				// return error (could be a permission problem)
				return err
			case info == nil:
				// this should only happen when err != nil but let's be sure
				return nil
			case info.IsDir():
				if path != wrapper.configDirPath {
					// make sure to not recurse into sub-directories
					return filepath.SkipDir
				}
				// ignore directories
				return nil
			default:
				// only add *.conf files
				if strings.HasSuffix(path, ".conf") {
					configs = append(configs, path)
				}
				return nil
			}
		},
	)

	if err != nil && !os.IsNotExist(err) {
		// Ignore IsNotExist errors: most systems won't have a registries.conf.d
		// directory.
		return nil, errors.Wrapf(err, "error reading registries.conf.d")
	}

	return configs, nil
}

// TryUpdatingCache loads the configuration from the provided `SystemContext`
// without using the internal cache. On success, the loaded configuration will
// be added into the internal registry cache.
func TryUpdatingCache(ctx *types.SystemContext) (*V2RegistriesConf, error) {
	return tryUpdatingCache(ctx, newConfigWrapper(ctx))
}

// tryUpdatingCache implements TryUpdatingCache with an additional configWrapper
// argument to avoid redundantly calculating the config paths.
func tryUpdatingCache(ctx *types.SystemContext, wrapper configWrapper) (*V2RegistriesConf, error) {
	configMutex.Lock()
	defer configMutex.Unlock()

	// load the config
	config := &tomlConfig{}
	if err := config.loadConfig(wrapper.configPath, false); err != nil {
		// Continue with an empty []Registry if we use the default config, which
		// implies that the config path of the SystemContext isn't set.
		//
		// Note: if ctx.SystemRegistriesConfPath points to the default config,
		// we will still return an error.
		if os.IsNotExist(err) && (ctx == nil || ctx.SystemRegistriesConfPath == "") {
			config = &tomlConfig{}
			config.V2RegistriesConf = V2RegistriesConf{Registries: []Registry{}}
		} else {
			return nil, errors.Wrapf(err, "error loading registries configuration %q", wrapper.configPath)
		}
	}

	// Load the configs from the conf directory path.
	dinConfigs, err := dropInConfigs(wrapper)
	if err != nil {
		return nil, err
	}
	for _, path := range dinConfigs {
		// Enforce v2 format for drop-in-configs.
		if err := config.loadConfig(path, true); err != nil {
			return nil, errors.Wrapf(err, "error loading drop-in registries configuration %q", path)
		}
	}

	v2Config := &config.V2RegistriesConf

	// populate the cache
	configCache[wrapper] = v2Config
	return v2Config, nil
}

// GetRegistries loads and returns the registries specified in the config.
// Note the parsed content of registry config files is cached.  For reloading,
// use `InvalidateCache` and re-call `GetRegistries`.
func GetRegistries(ctx *types.SystemContext) ([]Registry, error) {
	config, err := getConfig(ctx)
	if err != nil {
		return nil, err
	}
	return config.Registries, nil
}

// UnqualifiedSearchRegistries returns a list of host[:port] entries to try
// for unqualified image search, in the returned order)
func UnqualifiedSearchRegistries(ctx *types.SystemContext) ([]string, error) {
	config, err := getConfig(ctx)
	if err != nil {
		return nil, err
	}
	return config.UnqualifiedSearchRegistries, nil
}

// refMatchesPrefix returns true iff ref,
// which is a registry, repository namespace, repository or image reference (as formatted by
// reference.Domain(), reference.Named.Name() or reference.Reference.String()
// — note that this requires the name to start with an explicit hostname!),
// matches a Registry.Prefix value.
// (This is split from the caller primarily to make testing easier.)
func refMatchesPrefix(ref, prefix string) bool {
	switch {
	case len(ref) < len(prefix):
		return false
	case len(ref) == len(prefix):
		return ref == prefix
	case len(ref) > len(prefix):
		if !strings.HasPrefix(ref, prefix) {
			return false
		}
		c := ref[len(prefix)]
		// This allows "example.com:5000" to match "example.com",
		// which is unintended; that will get fixed eventually, DON'T RELY
		// ON THE CURRENT BEHAVIOR.
		return c == ':' || c == '/' || c == '@'
	default:
		panic("Internal error: impossible comparison outcome")
	}
}

// FindRegistry returns the Registry with the longest prefix for ref,
// which is a registry, repository namespace repository or image reference (as formatted by
// reference.Domain(), reference.Named.Name() or reference.Reference.String()
// — note that this requires the name to start with an explicit hostname!).
// If no Registry prefixes the image, nil is returned.
func FindRegistry(ctx *types.SystemContext, ref string) (*Registry, error) {
	config, err := getConfig(ctx)
	if err != nil {
		return nil, err
	}

	reg := Registry{}
	prefixLen := 0
	for _, r := range config.Registries {
		if refMatchesPrefix(ref, r.Prefix) {
			length := len(r.Prefix)
			if length > prefixLen {
				reg = r
				prefixLen = length
			}
		}
	}
	if prefixLen != 0 {
		return &reg, nil
	}
	return nil, nil
}

// loadConfig loads and unmarshals the configuration at the specified path. Note
// that v1 configs are translated into v2 and are cleared.  Use forceV2 if the
// config must in the v2 format.
//
// Note that specified fields in path will replace already set fields in the
// tomlConfig.  Only the [[registry]] tables are merged by prefix.
func (c *tomlConfig) loadConfig(path string, forceV2 bool) error {
	logrus.Debugf("Loading registries configuration %q", path)

	// Save the registries before decoding the file where they could be lost.
	// We merge them later again.
	registryMap := make(map[string]Registry)
	for i := range c.Registries {
		registryMap[c.Registries[i].Prefix] = c.Registries[i]
	}

	// Load the tomlConfig. Note that `DecodeFile` will overwrite set fields.
	c.Registries = nil // important to clear the memory to prevent us from overlapping fields
	_, err := toml.DecodeFile(path, c)
	if err != nil {
		return err
	}

	if c.V1RegistriesConf.Nonempty() {
		// Enforce the v2 format if requested.
		if forceV2 {
			return &InvalidRegistries{s: "registry must be in v2 format but is in v1"}
		}

		// Convert a v1 config into a v2 config.
		if c.V2RegistriesConf.Nonempty() {
			return &InvalidRegistries{s: "mixing sysregistry v1/v2 is not supported"}
		}
		v2, err := c.V1RegistriesConf.ConvertToV2()
		if err != nil {
			return err
		}
		c.V1RegistriesConf = V1RegistriesConf{}
		c.V2RegistriesConf = *v2
	}

	// Post process registries, set the correct prefixes, sanity checks, etc.
	if err := c.postProcess(); err != nil {
		return err
	}

	// Merge the freshly loaded registries.
	for i := range c.Registries {
		registryMap[c.Registries[i].Prefix] = c.Registries[i]
	}

	// Go maps have a non-deterministic order when iterating the keys, so
	// we dump them in a slice and sort it to enforce some order in
	// Registries slice.  Some consumers of c/image (e.g., CRI-O) log the
	// the configuration where a non-deterministic order could easily cause
	// confusion.
	prefixes := []string{}
	for prefix := range registryMap {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)

	c.Registries = []Registry{}
	for _, prefix := range prefixes {
		c.Registries = append(c.Registries, registryMap[prefix])
	}

	return nil
}
