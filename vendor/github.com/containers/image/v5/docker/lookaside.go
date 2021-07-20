package docker

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/internal/rootless"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage/pkg/homedir"
	"github.com/ghodss/yaml"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// systemRegistriesDirPath is the path to registries.d, used for locating lookaside Docker signature storage.
// You can override this at build time with
// -ldflags '-X github.com/containers/image/v5/docker.systemRegistriesDirPath=$your_path'
var systemRegistriesDirPath = builtinRegistriesDirPath

// builtinRegistriesDirPath is the path to registries.d.
// DO NOT change this, instead see systemRegistriesDirPath above.
const builtinRegistriesDirPath = "/etc/containers/registries.d"

// userRegistriesDirPath is the path to the per user registries.d.
var userRegistriesDir = filepath.FromSlash(".config/containers/registries.d")

// defaultUserDockerDir is the default sigstore directory for unprivileged user
var defaultUserDockerDir = filepath.FromSlash(".local/share/containers/sigstore")

// defaultDockerDir is the default sigstore directory for root
var defaultDockerDir = "/var/lib/containers/sigstore"

// registryConfiguration is one of the files in registriesDirPath configuring lookaside locations, or the result of merging them all.
// NOTE: Keep this in sync with docs/registries.d.md!
type registryConfiguration struct {
	DefaultDocker *registryNamespace `json:"default-docker"`
	// The key is a namespace, using fully-expanded Docker reference format or parent namespaces (per dockerReference.PolicyConfiguration*),
	Docker map[string]registryNamespace `json:"docker"`
}

// registryNamespace defines lookaside locations for a single namespace.
type registryNamespace struct {
	SigStore        string `json:"sigstore"`         // For reading, and if SigStoreStaging is not present, for writing.
	SigStoreStaging string `json:"sigstore-staging"` // For writing only.
}

// signatureStorageBase is an "opaque" type representing a lookaside Docker signature storage.
// Users outside of this file should use SignatureStorageBaseURL and signatureStorageURL below.
type signatureStorageBase *url.URL

// SignatureStorageBaseURL reads configuration to find an appropriate signature storage URL for ref, for write access if “write”.
// the usage of the BaseURL is defined under docker/distribution registries—separate storage of docs/signature-protocols.md
// Warning: This function only exposes configuration in registries.d;
// just because this function returns an URL does not mean that the URL will be used by c/image/docker (e.g. if the registry natively supports X-R-S-S).
func SignatureStorageBaseURL(sys *types.SystemContext, ref types.ImageReference, write bool) (*url.URL, error) {
	dr, ok := ref.(dockerReference)
	if !ok {
		return nil, errors.Errorf("ref must be a dockerReference")
	}
	// FIXME? Loading and parsing the config could be cached across calls.
	dirPath := registriesDirPath(sys)
	logrus.Debugf(`Using registries.d directory %s for sigstore configuration`, dirPath)
	config, err := loadAndMergeConfig(dirPath)
	if err != nil {
		return nil, err
	}

	topLevel := config.signatureTopLevel(dr, write)
	var url *url.URL
	if topLevel != "" {
		url, err = url.Parse(topLevel)
		if err != nil {
			return nil, errors.Wrapf(err, "Invalid signature storage URL %s", topLevel)
		}
	} else {
		// returns default directory if no sigstore specified in configuration file
		url = builtinDefaultSignatureStorageDir(rootless.GetRootlessEUID())
		logrus.Debugf(" No signature storage configuration found for %s, using built-in default %s", dr.PolicyConfigurationIdentity(), url.String())
	}
	// NOTE: Keep this in sync with docs/signature-protocols.md!
	// FIXME? Restrict to explicitly supported schemes?
	repo := reference.Path(dr.ref) // Note that this is without a tag or digest.
	if path.Clean(repo) != repo {  // Coverage: This should not be reachable because /./ and /../ components are not valid in docker references
		return nil, errors.Errorf("Unexpected path elements in Docker reference %s for signature storage", dr.ref.String())
	}
	url.Path = url.Path + "/" + repo
	return url, nil
}

// registriesDirPath returns a path to registries.d
func registriesDirPath(sys *types.SystemContext) string {
	return registriesDirPathWithHomeDir(sys, homedir.Get())
}

// registriesDirPathWithHomeDir is an internal implementation detail of registriesDirPath,
// it exists only to allow testing it with an artificial home directory.
func registriesDirPathWithHomeDir(sys *types.SystemContext, homeDir string) string {
	if sys != nil && sys.RegistriesDirPath != "" {
		return sys.RegistriesDirPath
	}
	userRegistriesDirPath := filepath.Join(homeDir, userRegistriesDir)
	if _, err := os.Stat(userRegistriesDirPath); err == nil {
		return userRegistriesDirPath
	}
	if sys != nil && sys.RootForImplicitAbsolutePaths != "" {
		return filepath.Join(sys.RootForImplicitAbsolutePaths, systemRegistriesDirPath)
	}

	return systemRegistriesDirPath
}

// builtinDefaultSignatureStorageDir returns default signature storage URL as per euid
func builtinDefaultSignatureStorageDir(euid int) *url.URL {
	if euid != 0 {
		return &url.URL{Scheme: "file", Path: filepath.Join(homedir.Get(), defaultUserDockerDir)}
	}
	return &url.URL{Scheme: "file", Path: defaultDockerDir}
}

// loadAndMergeConfig loads configuration files in dirPath
func loadAndMergeConfig(dirPath string) (*registryConfiguration, error) {
	mergedConfig := registryConfiguration{Docker: map[string]registryNamespace{}}
	dockerDefaultMergedFrom := ""
	nsMergedFrom := map[string]string{}

	dir, err := os.Open(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &mergedConfig, nil
		}
		return nil, err
	}
	configNames, err := dir.Readdirnames(0)
	if err != nil {
		return nil, err
	}
	for _, configName := range configNames {
		if !strings.HasSuffix(configName, ".yaml") {
			continue
		}
		configPath := filepath.Join(dirPath, configName)
		configBytes, err := ioutil.ReadFile(configPath)
		if err != nil {
			return nil, err
		}

		var config registryConfiguration
		err = yaml.Unmarshal(configBytes, &config)
		if err != nil {
			return nil, errors.Wrapf(err, "parsing %s", configPath)
		}

		if config.DefaultDocker != nil {
			if mergedConfig.DefaultDocker != nil {
				return nil, errors.Errorf(`Error parsing signature storage configuration: "default-docker" defined both in "%s" and "%s"`,
					dockerDefaultMergedFrom, configPath)
			}
			mergedConfig.DefaultDocker = config.DefaultDocker
			dockerDefaultMergedFrom = configPath
		}

		for nsName, nsConfig := range config.Docker { // includes config.Docker == nil
			if _, ok := mergedConfig.Docker[nsName]; ok {
				return nil, errors.Errorf(`Error parsing signature storage configuration: "docker" namespace "%s" defined both in "%s" and "%s"`,
					nsName, nsMergedFrom[nsName], configPath)
			}
			mergedConfig.Docker[nsName] = nsConfig
			nsMergedFrom[nsName] = configPath
		}
	}

	return &mergedConfig, nil
}

// config.signatureTopLevel returns an URL string configured in config for ref, for write access if “write”.
// (the top level of the storage, namespaced by repo.FullName etc.), or "" if nothing has been configured.
func (config *registryConfiguration) signatureTopLevel(ref dockerReference, write bool) string {
	if config.Docker != nil {
		// Look for a full match.
		identity := ref.PolicyConfigurationIdentity()
		if ns, ok := config.Docker[identity]; ok {
			logrus.Debugf(` Using "docker" namespace %s`, identity)
			if url := ns.signatureTopLevel(write); url != "" {
				return url
			}
		}

		// Look for a match of the possible parent namespaces.
		for _, name := range ref.PolicyConfigurationNamespaces() {
			if ns, ok := config.Docker[name]; ok {
				logrus.Debugf(` Using "docker" namespace %s`, name)
				if url := ns.signatureTopLevel(write); url != "" {
					return url
				}
			}
		}
	}
	// Look for a default location
	if config.DefaultDocker != nil {
		logrus.Debugf(` Using "default-docker" configuration`)
		if url := config.DefaultDocker.signatureTopLevel(write); url != "" {
			return url
		}
	}
	return ""
}

// ns.signatureTopLevel returns an URL string configured in ns for ref, for write access if “write”.
// or "" if nothing has been configured.
func (ns registryNamespace) signatureTopLevel(write bool) string {
	if write && ns.SigStoreStaging != "" {
		logrus.Debugf(`  Using %s`, ns.SigStoreStaging)
		return ns.SigStoreStaging
	}
	if ns.SigStore != "" {
		logrus.Debugf(`  Using %s`, ns.SigStore)
		return ns.SigStore
	}
	return ""
}

// signatureStorageURL returns an URL usable for accessing signature index in base with known manifestDigest.
// base is not nil from the caller
// NOTE: Keep this in sync with docs/signature-protocols.md!
func signatureStorageURL(base signatureStorageBase, manifestDigest digest.Digest, index int) *url.URL {
	url := *base
	url.Path = fmt.Sprintf("%s@%s=%s/signature-%d", url.Path, manifestDigest.Algorithm(), manifestDigest.Hex(), index+1)
	return &url
}
