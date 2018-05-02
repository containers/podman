package sysregistriesv2

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/containers/image/types"
)

// systemRegistriesConfPath is the path to the system-wide registry
// configuration file and is used to add/subtract potential registries for
// obtaining images.  You can override this at build time with
// -ldflags '-X github.com/containers/image/sysregistries.systemRegistriesConfPath=$your_path'
var systemRegistriesConfPath = builtinRegistriesConfPath

// builtinRegistriesConfPath is the path to the registry configuration file.
// DO NOT change this, instead see systemRegistriesConfPath above.
const builtinRegistriesConfPath = "/etc/containers/registries.conf"

// tomlURL is an abstraction required to unmarshal the non-primitive and
// non-local url.URL type when loading the toml config.
type tomlURL struct {
	url url.URL
}

// UnmashalText interprets and parses text as a net.URL type and assigns it to
// the tomlURL's url field.
func (r *tomlURL) UnmarshalText(text []byte) (err error) {
	r.url, err = parseURL(string(text))
	return err
}

// mirror is an internal type including all mirror data but the URL.
type mirror struct {
	// If true, certs verification will be skipped and HTTP (non-TLS)
	// connections will be allowed.
	Insecure bool `toml:"insecure"`
}

// Mirror represents a mirror. Mirrors can be used as pull-through caches for
// registries.
type Mirror struct {
	// The mirror's URL.
	URL url.URL
	mirror
}

// tomlMirror is a serializable Mirror.
type tomlMirror struct {
	// Serializable mirror URL.
	URL tomlURL `toml:"url"`
	mirror
}

// toMirror transforms tmirror to a Mirror.
func (tmir *tomlMirror) toMirror() (Mirror, error) {
	if len(tmir.URL.url.String()) == 0 {
		return Mirror{}, fmt.Errorf("mirror must include a URL")
	}
	mir := Mirror{URL: tmir.URL.url, mirror: tmir.mirror}
	return mir, nil
}

// registry is an internal type including all registry data but the URL and the
// array of associated mirrors.
type registry struct {
	// If true, pulling from the registry will be blocked.
	Blocked bool `toml:"blocked"`
	// If true, certs verification will be skipped and HTTP (non-TLS)
	// connections will be allowed.
	Insecure bool `toml:"insecure"`
	// If true, the registry can be used when pulling an unqualified image.
	Search bool `toml:"unqualified-search"`
	// Prefix is used for matching images, and to translate one namespace to
	// another.  If `Prefix="example.com/bar"`, `URL="https://example.com/foo/bar"`
	// and we pull from "example.com/bar/myimage:latest", the image will
	// effectively be pulled from https://example.com/foo/bar/myimage:latest.
	// If no Prefix is specified, it defaults to the specified URL.
	Prefix string `toml:"prefix"`
}

// Registry represents a registry.
type Registry struct {
	// Serializable registry URL.
	URL url.URL
	// The registry's mirrors.
	Mirrors []Mirror
	registry
}

// tomlRegistry is serializable Registry.
type tomlRegistry struct {
	URL     tomlURL      `toml:"url"`
	Mirrors []tomlMirror `toml:"mirror"`
	registry
}

// stripURIScheme strips the URI scheme from the given URL and returns it as
// as string.
func stripURIScheme(url url.URL) string {
	return strings.TrimPrefix(url.String(), url.Scheme+"://")
}

// toRegistry transforms treg to a Registry.
func (treg *tomlRegistry) toRegistry() (Registry, error) {
	if len(treg.URL.url.String()) == 0 {
		return Registry{}, fmt.Errorf("registry must include a URL")
	}
	reg := Registry{URL: treg.URL.url, registry: treg.registry}
	// if no prefix is specified, default to the specified URL with
	// stripped URI scheme
	if reg.Prefix == "" {
		reg.Prefix = stripURIScheme(reg.URL)
	}

	for _, tmir := range treg.Mirrors {
		mir, err := tmir.toMirror()
		if err != nil {
			return Registry{}, err
		}
		reg.Mirrors = append(reg.Mirrors, mir)
	}
	return reg, nil
}

// backwards compatability to sysregistries v1
type v1TOMLregistries struct {
	Registries []string `toml:"registries"`
}

// tomlConfig is the data type used to unmarshal the toml config.
type tomlConfig struct {
	TOMLRegistries []tomlRegistry `toml:"registry"`
	// backwards compatability to sysregistries v1
	V1Registries struct {
		Search   v1TOMLregistries `toml:"search"`
		Insecure v1TOMLregistries `toml:"insecure"`
		Block    v1TOMLregistries `toml:"block"`
	} `toml:"registries"`
}

// parseURL parses the input string, performs some sanity checks and returns
// a url.URL.  The input must be a valid URI with an "http" or "https" scheme,
// a specified host and an empty URI user.  Otherwise, an error is returned.
func parseURL(input string) (url.URL, error) {
	input = strings.TrimRight(input, "/")

	uri, err := url.Parse(input)
	if err != nil {
		return url.URL{}, fmt.Errorf("error parsing URL %s: %v", input, err)
	}

	// only https and http are valid URI schemes
	if uri.Scheme == "" {
		return url.URL{}, fmt.Errorf("unspecified URI scheme: %s", input)
	}
	if uri.Scheme != "https" && uri.Scheme != "http" {
		return url.URL{}, fmt.Errorf("unsupported URI scheme: %s", input)
	}

	// a host must be specified
	if uri.Host == "" {
		return url.URL{}, fmt.Errorf("unspecified URI host: %s", input)
	}

	// user must be empty
	if uri.User != nil {
		// strip password for security reasons
		uri.User = url.UserPassword(uri.User.Username(), "xxxxxx")
		return url.URL{}, fmt.Errorf("unsupported username/password: %q", uri)
	}

	return *uri, nil
}

// getV1Registries transforms v1 registries in the config into an array of v2
// registries of type Registry.
func getV1Registries(config *tomlConfig) ([]Registry, error) {
	regMap := make(map[string]*Registry)

	getRegistry := func(s string) (*Registry, error) { // Note: _pointer_ to a long-lived object
		url, err := parseURL(s)
		if err != nil {
			return nil, err
		}
		prefix := stripURIScheme(url)
		reg, exists := regMap[prefix]
		if !exists {
			reg = &Registry{URL: url,
				Mirrors:  []Mirror{},
				registry: registry{Prefix: prefix}}
			regMap[prefix] = reg
		}
		return reg, nil
	}

	for _, search := range config.V1Registries.Search.Registries {
		reg, err := getRegistry(search)
		if err != nil {
			return nil, err
		}
		reg.Search = true
	}
	for _, blocked := range config.V1Registries.Block.Registries {
		reg, err := getRegistry(blocked)
		if err != nil {
			return nil, err
		}
		reg.Blocked = true
	}
	for _, insecure := range config.V1Registries.Insecure.Registries {
		reg, err := getRegistry(insecure)
		if err != nil {
			return nil, err
		}
		reg.Insecure = true
	}

	registries := []Registry{}
	for _, reg := range regMap {
		registries = append(registries, *reg)
	}
	return registries, nil
}

// GetRegistries loads and returns the registries specified in the config.
func GetRegistries(ctx *types.SystemContext) ([]Registry, error) {
	config, err := loadRegistryConf(ctx)
	if err != nil {
		return nil, err
	}

	registries := []Registry{}
	for _, treg := range config.TOMLRegistries {
		reg, err := treg.toRegistry()
		if err != nil {
			return nil, err
		}
		registries = append(registries, reg)
	}

	// backwards compatibility for v1 configs
	v1Registries, err := getV1Registries(config)
	if err != nil {
		return nil, err
	}
	if len(v1Registries) > 0 {
		if len(registries) > 0 {
			return nil, fmt.Errorf("mixing sysregistry v1/v2 is not supported")
		}
		registries = v1Registries
	}

	return registries, nil
}

// FindUnqualifiedSearchRegistries returns all registries that are configured
// for unqualified image search (i.e., with Registry.Search == true).
func FindUnqualifiedSearchRegistries(registries []Registry) []Registry {
	unqualified := []Registry{}
	for _, reg := range registries {
		if reg.Search {
			unqualified = append(unqualified, reg)
		}
	}
	return unqualified
}

// FindRegistry returns the Registry with the longest prefix for ref.  If no
// Registry prefixes the image, nil is returned.
func FindRegistry(ref string, registries []Registry) *Registry {
	reg := Registry{}
	prefixLen := 0
	for _, r := range registries {
		if strings.HasPrefix(ref, r.Prefix) {
			length := len(r.Prefix)
			if length > prefixLen {
				reg = r
				prefixLen = length
			}
		}
	}
	if prefixLen != 0 {
		return &reg
	}
	return nil
}

// Reads the global registry file from the filesystem. Returns a byte array.
func readRegistryConf(ctx *types.SystemContext) ([]byte, error) {
	dirPath := systemRegistriesConfPath
	if ctx != nil {
		if ctx.SystemRegistriesConfPath != "" {
			dirPath = ctx.SystemRegistriesConfPath
		} else if ctx.RootForImplicitAbsolutePaths != "" {
			dirPath = filepath.Join(ctx.RootForImplicitAbsolutePaths, systemRegistriesConfPath)
		}
	}
	configBytes, err := ioutil.ReadFile(dirPath)
	return configBytes, err
}

// Used in unittests to parse custom configs without a types.SystemContext.
var readConf = readRegistryConf

// Loads the registry configuration file from the filesystem and then unmarshals
// it.  Returns the unmarshalled object.
func loadRegistryConf(ctx *types.SystemContext) (*tomlConfig, error) {
	config := &tomlConfig{}

	configBytes, err := readConf(ctx)
	if err != nil {
		return nil, err
	}

	err = toml.Unmarshal(configBytes, &config)
	return config, err
}
