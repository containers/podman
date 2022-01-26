package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/pkg/sysregistriesv2"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage/pkg/homedir"
	helperclient "github.com/docker/docker-credential-helpers/client"
	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type dockerAuthConfig struct {
	Auth          string `json:"auth,omitempty"`
	IdentityToken string `json:"identitytoken,omitempty"`
}

type dockerConfigFile struct {
	AuthConfigs map[string]dockerAuthConfig `json:"auths"`
	CredHelpers map[string]string           `json:"credHelpers,omitempty"`
}

type authPath struct {
	path         string
	legacyFormat bool
}

var (
	defaultPerUIDPathFormat = filepath.FromSlash("/run/containers/%d/auth.json")
	xdgConfigHomePath       = filepath.FromSlash("containers/auth.json")
	xdgRuntimeDirPath       = filepath.FromSlash("containers/auth.json")
	dockerHomePath          = filepath.FromSlash(".docker/config.json")
	dockerLegacyHomePath    = ".dockercfg"
	nonLinuxAuthFilePath    = filepath.FromSlash(".config/containers/auth.json")

	// ErrNotLoggedIn is returned for users not logged into a registry
	// that they are trying to logout of
	ErrNotLoggedIn = errors.New("not logged in")
	// ErrNotSupported is returned for unsupported methods
	ErrNotSupported = errors.New("not supported")
)

// SetCredentials stores the username and password in a location
// appropriate for sys and the users’ configuration.
// A valid key is a repository, a namespace within a registry, or a registry hostname;
// using forms other than just a registry may fail depending on configuration.
// Returns a human-redable description of the location that was updated.
// NOTE: The return value is only intended to be read by humans; its form is not an API,
// it may change (or new forms can be added) any time.
func SetCredentials(sys *types.SystemContext, key, username, password string) (string, error) {
	isNamespaced, err := validateKey(key)
	if err != nil {
		return "", err
	}

	helpers, err := sysregistriesv2.CredentialHelpers(sys)
	if err != nil {
		return "", err
	}

	// Make sure to collect all errors.
	var multiErr error
	for _, helper := range helpers {
		var desc string
		var err error
		switch helper {
		// Special-case the built-in helpers for auth files.
		case sysregistriesv2.AuthenticationFileHelper:
			desc, err = modifyJSON(sys, func(auths *dockerConfigFile) (bool, error) {
				if ch, exists := auths.CredHelpers[key]; exists {
					if isNamespaced {
						return false, unsupportedNamespaceErr(ch)
					}
					return false, setAuthToCredHelper(ch, key, username, password)
				}
				creds := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
				newCreds := dockerAuthConfig{Auth: creds}
				auths.AuthConfigs[key] = newCreds
				return true, nil
			})
		// External helpers.
		default:
			if isNamespaced {
				err = unsupportedNamespaceErr(helper)
			} else {
				desc = fmt.Sprintf("credential helper: %s", helper)
				err = setAuthToCredHelper(helper, key, username, password)
			}
		}
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
			logrus.Debugf("Error storing credentials for %s in credential helper %s: %v", key, helper, err)
			continue
		}
		logrus.Debugf("Stored credentials for %s in credential helper %s", key, helper)
		return desc, nil
	}
	return "", multiErr
}

func unsupportedNamespaceErr(helper string) error {
	return errors.Errorf("namespaced key is not supported for credential helper %s", helper)
}

// SetAuthentication stores the username and password in the credential helper or file
// See the documentation of SetCredentials for format of "key"
func SetAuthentication(sys *types.SystemContext, key, username, password string) error {
	_, err := SetCredentials(sys, key, username, password)
	return err
}

// GetAllCredentials returns the registry credentials for all registries stored
// in any of the configured credential helpers.
func GetAllCredentials(sys *types.SystemContext) (map[string]types.DockerAuthConfig, error) {
	// To keep things simple, let's first extract all registries from all
	// possible sources, and then call `GetCredentials` on them.  That
	// prevents us from having to reverse engineer the logic in
	// `GetCredentials`.
	allKeys := make(map[string]bool)
	addKey := func(s string) {
		allKeys[s] = true
	}

	// To use GetCredentials, we must at least convert the URL forms into host names.
	// While we're at it, we’ll also canonicalize docker.io to the standard format.
	normalizedDockerIORegistry := normalizeRegistry("docker.io")

	helpers, err := sysregistriesv2.CredentialHelpers(sys)
	if err != nil {
		return nil, err
	}
	for _, helper := range helpers {
		switch helper {
		// Special-case the built-in helper for auth files.
		case sysregistriesv2.AuthenticationFileHelper:
			for _, path := range getAuthFilePaths(sys, homedir.Get()) {
				// readJSONFile returns an empty map in case the path doesn't exist.
				auths, err := readJSONFile(path.path, path.legacyFormat)
				if err != nil {
					return nil, errors.Wrapf(err, "reading JSON file %q", path.path)
				}
				// Credential helpers in the auth file have a
				// direct mapping to a registry, so we can just
				// walk the map.
				for registry := range auths.CredHelpers {
					addKey(registry)
				}
				for key := range auths.AuthConfigs {
					key := normalizeAuthFileKey(key, path.legacyFormat)
					if key == normalizedDockerIORegistry {
						key = "docker.io"
					}
					addKey(key)
				}
			}
		// External helpers.
		default:
			creds, err := listAuthsFromCredHelper(helper)
			if err != nil {
				logrus.Debugf("Error listing credentials stored in credential helper %s: %v", helper, err)
			}
			switch errors.Cause(err) {
			case nil:
				for registry := range creds {
					addKey(registry)
				}
			case exec.ErrNotFound:
				// It's okay if the helper doesn't exist.
			default:
				return nil, err
			}
		}
	}

	// Now use `GetCredentials` to the specific auth configs for each
	// previously listed registry.
	authConfigs := make(map[string]types.DockerAuthConfig)
	for key := range allKeys {
		authConf, err := GetCredentials(sys, key)
		if err != nil {
			// Note: we rely on the logging in `GetCredentials`.
			return nil, err
		}
		if authConf != (types.DockerAuthConfig{}) {
			authConfigs[key] = authConf
		}
	}

	return authConfigs, nil
}

// getAuthFilePaths returns a slice of authPaths based on the system context
// in the order they should be searched. Note that some paths may not exist.
// The homeDir parameter should always be homedir.Get(), and is only intended to be overridden
// by tests.
func getAuthFilePaths(sys *types.SystemContext, homeDir string) []authPath {
	paths := []authPath{}
	pathToAuth, lf, err := getPathToAuth(sys)
	if err == nil {
		paths = append(paths, authPath{path: pathToAuth, legacyFormat: lf})
	} else {
		// Error means that the path set for XDG_RUNTIME_DIR does not exist
		// but we don't want to completely fail in the case that the user is pulling a public image
		// Logging the error as a warning instead and moving on to pulling the image
		logrus.Warnf("%v: Trying to pull image in the event that it is a public image.", err)
	}
	xdgCfgHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgCfgHome == "" {
		xdgCfgHome = filepath.Join(homeDir, ".config")
	}
	paths = append(paths, authPath{path: filepath.Join(xdgCfgHome, xdgConfigHomePath), legacyFormat: false})
	if dockerConfig := os.Getenv("DOCKER_CONFIG"); dockerConfig != "" {
		paths = append(paths,
			authPath{path: filepath.Join(dockerConfig, "config.json"), legacyFormat: false},
		)
	} else {
		paths = append(paths,
			authPath{path: filepath.Join(homeDir, dockerHomePath), legacyFormat: false},
		)
	}
	paths = append(paths,
		authPath{path: filepath.Join(homeDir, dockerLegacyHomePath), legacyFormat: true},
	)
	return paths
}

// GetCredentials returns the registry credentials matching key, appropriate for
// sys and the users’ configuration.
// If an entry is not found, an empty struct is returned.
// A valid key is a repository, a namespace within a registry, or a registry hostname.
//
// GetCredentialsForRef should almost always be used in favor of this API.
func GetCredentials(sys *types.SystemContext, key string) (types.DockerAuthConfig, error) {
	return getCredentialsWithHomeDir(sys, key, homedir.Get())
}

// GetCredentialsForRef returns the registry credentials necessary for
// accessing ref on the registry ref points to,
// appropriate for sys and the users’ configuration.
// If an entry is not found, an empty struct is returned.
func GetCredentialsForRef(sys *types.SystemContext, ref reference.Named) (types.DockerAuthConfig, error) {
	return getCredentialsWithHomeDir(sys, ref.Name(), homedir.Get())
}

// getCredentialsWithHomeDir is an internal implementation detail of
// GetCredentialsForRef and GetCredentials. It exists only to allow testing it
// with an artificial home directory.
func getCredentialsWithHomeDir(sys *types.SystemContext, key, homeDir string) (types.DockerAuthConfig, error) {
	_, err := validateKey(key)
	if err != nil {
		return types.DockerAuthConfig{}, err
	}

	if sys != nil && sys.DockerAuthConfig != nil {
		logrus.Debugf("Returning credentials for %s from DockerAuthConfig", key)
		return *sys.DockerAuthConfig, nil
	}

	var registry string // We compute this once because it is used in several places.
	if firstSlash := strings.IndexRune(key, '/'); firstSlash != -1 {
		registry = key[:firstSlash]
	} else {
		registry = key
	}

	// Anonymous function to query credentials from auth files.
	getCredentialsFromAuthFiles := func() (types.DockerAuthConfig, string, error) {
		for _, path := range getAuthFilePaths(sys, homeDir) {
			authConfig, err := findCredentialsInFile(key, registry, path.path, path.legacyFormat)
			if err != nil {
				return types.DockerAuthConfig{}, "", err
			}

			if authConfig != (types.DockerAuthConfig{}) {
				return authConfig, path.path, nil
			}
		}
		return types.DockerAuthConfig{}, "", nil
	}

	helpers, err := sysregistriesv2.CredentialHelpers(sys)
	if err != nil {
		return types.DockerAuthConfig{}, err
	}

	var multiErr error
	for _, helper := range helpers {
		var (
			creds          types.DockerAuthConfig
			helperKey      string
			credHelperPath string
			err            error
		)
		switch helper {
		// Special-case the built-in helper for auth files.
		case sysregistriesv2.AuthenticationFileHelper:
			helperKey = key
			creds, credHelperPath, err = getCredentialsFromAuthFiles()
		// External helpers.
		default:
			// This intentionally uses "registry", not "key"; we don't support namespaced
			// credentials in helpers, but a "registry" is a valid parent of "key".
			helperKey = registry
			creds, err = getAuthFromCredHelper(helper, registry)
		}
		if err != nil {
			logrus.Debugf("Error looking up credentials for %s in credential helper %s: %v", helperKey, helper, err)
			multiErr = multierror.Append(multiErr, err)
			continue
		}
		if creds != (types.DockerAuthConfig{}) {
			msg := fmt.Sprintf("Found credentials for %s in credential helper %s", helperKey, helper)
			if credHelperPath != "" {
				msg = fmt.Sprintf("%s in file %s", msg, credHelperPath)
			}
			logrus.Debug(msg)
			return creds, nil
		}
	}
	if multiErr != nil {
		return types.DockerAuthConfig{}, multiErr
	}

	logrus.Debugf("No credentials for %s found", key)
	return types.DockerAuthConfig{}, nil
}

// GetAuthentication returns the registry credentials matching key, appropriate for
// sys and the users’ configuration.
// If an entry is not found, an empty struct is returned.
// A valid key is a repository, a namespace within a registry, or a registry hostname.
//
// Deprecated: This API only has support for username and password. To get the
// support for oauth2 in container registry authentication, we added the new
// GetCredentialsForRef and GetCredentials API. The new API should be used and this API is kept to
// maintain backward compatibility.
func GetAuthentication(sys *types.SystemContext, key string) (string, string, error) {
	return getAuthenticationWithHomeDir(sys, key, homedir.Get())
}

// getAuthenticationWithHomeDir is an internal implementation detail of GetAuthentication,
// it exists only to allow testing it with an artificial home directory.
func getAuthenticationWithHomeDir(sys *types.SystemContext, key, homeDir string) (string, string, error) {
	auth, err := getCredentialsWithHomeDir(sys, key, homeDir)
	if err != nil {
		return "", "", err
	}
	if auth.IdentityToken != "" {
		return "", "", errors.Wrap(ErrNotSupported, "non-empty identity token found and this API doesn't support it")
	}
	return auth.Username, auth.Password, nil
}

// RemoveAuthentication removes credentials for `key` from all possible
// sources such as credential helpers and auth files.
// A valid key is a repository, a namespace within a registry, or a registry hostname;
// using forms other than just a registry may fail depending on configuration.
func RemoveAuthentication(sys *types.SystemContext, key string) error {
	isNamespaced, err := validateKey(key)
	if err != nil {
		return err
	}

	helpers, err := sysregistriesv2.CredentialHelpers(sys)
	if err != nil {
		return err
	}

	var multiErr error
	isLoggedIn := false

	removeFromCredHelper := func(helper string) {
		if isNamespaced {
			logrus.Debugf("Not removing credentials because namespaced keys are not supported for the credential helper: %s", helper)
			return
		} else {
			err := deleteAuthFromCredHelper(helper, key)
			if err == nil {
				logrus.Debugf("Credentials for %q were deleted from credential helper %s", key, helper)
				isLoggedIn = true
				return
			}
			if credentials.IsErrCredentialsNotFoundMessage(err.Error()) {
				logrus.Debugf("Not logged in to %s with credential helper %s", key, helper)
				return
			}
		}
		multiErr = multierror.Append(multiErr, errors.Wrapf(err, "removing credentials for %s from credential helper %s", key, helper))
	}

	for _, helper := range helpers {
		var err error
		switch helper {
		// Special-case the built-in helper for auth files.
		case sysregistriesv2.AuthenticationFileHelper:
			_, err = modifyJSON(sys, func(auths *dockerConfigFile) (bool, error) {
				if innerHelper, exists := auths.CredHelpers[key]; exists {
					removeFromCredHelper(innerHelper)
				}
				if _, ok := auths.AuthConfigs[key]; ok {
					isLoggedIn = true
					delete(auths.AuthConfigs, key)
				}
				return true, multiErr
			})
			if err != nil {
				multiErr = multierror.Append(multiErr, err)
			}
		// External helpers.
		default:
			removeFromCredHelper(helper)
		}
	}

	if multiErr != nil {
		return multiErr
	}
	if !isLoggedIn {
		return ErrNotLoggedIn
	}

	return nil
}

// RemoveAllAuthentication deletes all the credentials stored in credential
// helpers and auth files.
func RemoveAllAuthentication(sys *types.SystemContext) error {
	helpers, err := sysregistriesv2.CredentialHelpers(sys)
	if err != nil {
		return err
	}

	var multiErr error
	for _, helper := range helpers {
		var err error
		switch helper {
		// Special-case the built-in helper for auth files.
		case sysregistriesv2.AuthenticationFileHelper:
			_, err = modifyJSON(sys, func(auths *dockerConfigFile) (bool, error) {
				for registry, helper := range auths.CredHelpers {
					// Helpers in auth files are expected
					// to exist, so no special treatment
					// for them.
					if err := deleteAuthFromCredHelper(helper, registry); err != nil {
						return false, err
					}
				}
				auths.CredHelpers = make(map[string]string)
				auths.AuthConfigs = make(map[string]dockerAuthConfig)
				return true, nil
			})
		// External helpers.
		default:
			var creds map[string]string
			creds, err = listAuthsFromCredHelper(helper)
			switch errors.Cause(err) {
			case nil:
				for registry := range creds {
					err = deleteAuthFromCredHelper(helper, registry)
					if err != nil {
						break
					}
				}
			case exec.ErrNotFound:
				// It's okay if the helper doesn't exist.
				continue
			default:
				// fall through
			}
		}
		if err != nil {
			logrus.Debugf("Error removing credentials from credential helper %s: %v", helper, err)
			multiErr = multierror.Append(multiErr, err)
			continue
		}
		logrus.Debugf("All credentials removed from credential helper %s", helper)
	}

	return multiErr
}

func listAuthsFromCredHelper(credHelper string) (map[string]string, error) {
	helperName := fmt.Sprintf("docker-credential-%s", credHelper)
	p := helperclient.NewShellProgramFunc(helperName)
	return helperclient.List(p)
}

// getPathToAuth gets the path of the auth.json file used for reading and writing credentials
// returns the path, and a bool specifies whether the file is in legacy format
func getPathToAuth(sys *types.SystemContext) (string, bool, error) {
	return getPathToAuthWithOS(sys, runtime.GOOS)
}

// getPathToAuthWithOS is an internal implementation detail of getPathToAuth,
// it exists only to allow testing it with an artificial runtime.GOOS.
func getPathToAuthWithOS(sys *types.SystemContext, goOS string) (string, bool, error) {
	if sys != nil {
		if sys.AuthFilePath != "" {
			return sys.AuthFilePath, false, nil
		}
		if sys.LegacyFormatAuthFilePath != "" {
			return sys.LegacyFormatAuthFilePath, true, nil
		}
		if sys.RootForImplicitAbsolutePaths != "" {
			return filepath.Join(sys.RootForImplicitAbsolutePaths, fmt.Sprintf(defaultPerUIDPathFormat, os.Getuid())), false, nil
		}
	}
	if goOS == "windows" || goOS == "darwin" {
		return filepath.Join(homedir.Get(), nonLinuxAuthFilePath), false, nil
	}

	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir != "" {
		// This function does not in general need to separately check that the returned path exists; that’s racy, and callers will fail accessing the file anyway.
		// We are checking for os.IsNotExist here only to give the user better guidance what to do in this special case.
		_, err := os.Stat(runtimeDir)
		if os.IsNotExist(err) {
			// This means the user set the XDG_RUNTIME_DIR variable and either forgot to create the directory
			// or made a typo while setting the environment variable,
			// so return an error referring to $XDG_RUNTIME_DIR instead of xdgRuntimeDirPath inside.
			return "", false, errors.Wrapf(err, "%q directory set by $XDG_RUNTIME_DIR does not exist. Either create the directory or unset $XDG_RUNTIME_DIR.", runtimeDir)
		} // else ignore err and let the caller fail accessing xdgRuntimeDirPath.
		return filepath.Join(runtimeDir, xdgRuntimeDirPath), false, nil
	}
	return fmt.Sprintf(defaultPerUIDPathFormat, os.Getuid()), false, nil
}

// readJSONFile unmarshals the authentications stored in the auth.json file and returns it
// or returns an empty dockerConfigFile data structure if auth.json does not exist
// if the file exists and is empty, readJSONFile returns an error
func readJSONFile(path string, legacyFormat bool) (dockerConfigFile, error) {
	var auths dockerConfigFile

	raw, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			auths.AuthConfigs = map[string]dockerAuthConfig{}
			return auths, nil
		}
		return dockerConfigFile{}, err
	}

	if legacyFormat {
		if err = json.Unmarshal(raw, &auths.AuthConfigs); err != nil {
			return dockerConfigFile{}, errors.Wrapf(err, "unmarshaling JSON at %q", path)
		}
		return auths, nil
	}

	if err = json.Unmarshal(raw, &auths); err != nil {
		return dockerConfigFile{}, errors.Wrapf(err, "unmarshaling JSON at %q", path)
	}

	if auths.AuthConfigs == nil {
		auths.AuthConfigs = map[string]dockerAuthConfig{}
	}
	if auths.CredHelpers == nil {
		auths.CredHelpers = make(map[string]string)
	}

	return auths, nil
}

// modifyJSON finds an auth.json file, calls editor on the contents, and
// writes it back if editor returns true.
// Returns a human-redable description of the file, to be returned by SetCredentials.
func modifyJSON(sys *types.SystemContext, editor func(auths *dockerConfigFile) (bool, error)) (string, error) {
	path, legacyFormat, err := getPathToAuth(sys)
	if err != nil {
		return "", err
	}
	if legacyFormat {
		return "", fmt.Errorf("writes to %s using legacy format are not supported", path)
	}

	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}

	auths, err := readJSONFile(path, false)
	if err != nil {
		return "", errors.Wrapf(err, "reading JSON file %q", path)
	}

	updated, err := editor(&auths)
	if err != nil {
		return "", errors.Wrapf(err, "updating %q", path)
	}
	if updated {
		newData, err := json.MarshalIndent(auths, "", "\t")
		if err != nil {
			return "", errors.Wrapf(err, "marshaling JSON %q", path)
		}

		if err = ioutil.WriteFile(path, newData, 0600); err != nil {
			return "", errors.Wrapf(err, "writing to file %q", path)
		}
	}

	return path, nil
}

func getAuthFromCredHelper(credHelper, registry string) (types.DockerAuthConfig, error) {
	helperName := fmt.Sprintf("docker-credential-%s", credHelper)
	p := helperclient.NewShellProgramFunc(helperName)
	creds, err := helperclient.Get(p, registry)
	if err != nil {
		if credentials.IsErrCredentialsNotFoundMessage(err.Error()) {
			logrus.Debugf("Not logged in to %s with credential helper %s", registry, credHelper)
			err = nil
		}
		return types.DockerAuthConfig{}, err
	}

	switch creds.Username {
	case "<token>":
		return types.DockerAuthConfig{
			IdentityToken: creds.Secret,
		}, nil
	default:
		return types.DockerAuthConfig{
			Username: creds.Username,
			Password: creds.Secret,
		}, nil
	}
}

func setAuthToCredHelper(credHelper, registry, username, password string) error {
	helperName := fmt.Sprintf("docker-credential-%s", credHelper)
	p := helperclient.NewShellProgramFunc(helperName)
	creds := &credentials.Credentials{
		ServerURL: registry,
		Username:  username,
		Secret:    password,
	}
	return helperclient.Store(p, creds)
}

func deleteAuthFromCredHelper(credHelper, registry string) error {
	helperName := fmt.Sprintf("docker-credential-%s", credHelper)
	p := helperclient.NewShellProgramFunc(helperName)
	return helperclient.Erase(p, registry)
}

// findCredentialsInFile looks for credentials matching "key"
// (which is "registry" or a namespace in "registry") in "path".
func findCredentialsInFile(key, registry, path string, legacyFormat bool) (types.DockerAuthConfig, error) {
	auths, err := readJSONFile(path, legacyFormat)
	if err != nil {
		return types.DockerAuthConfig{}, errors.Wrapf(err, "reading JSON file %q", path)
	}

	// First try cred helpers. They should always be normalized.
	// This intentionally uses "registry", not "key"; we don't support namespaced
	// credentials in helpers.
	if ch, exists := auths.CredHelpers[registry]; exists {
		logrus.Debugf("Looking up in credential helper %s based on credHelpers entry in %s", ch, path)
		return getAuthFromCredHelper(ch, registry)
	}

	// Support sub-registry namespaces in auth.
	// (This is not a feature of ~/.docker/config.json; we support it even for
	// those files as an extension.)
	var keys []string
	if !legacyFormat {
		keys = authKeysForKey(key)
	} else {
		keys = []string{registry}
	}

	// Repo or namespace keys are only supported as exact matches. For registry
	// keys we prefer exact matches as well.
	for _, key := range keys {
		if val, exists := auths.AuthConfigs[key]; exists {
			return decodeDockerAuth(val)
		}
	}

	// bad luck; let's normalize the entries first
	// This primarily happens for legacyFormat, which for a time used API URLs
	// (http[s:]//…/v1/) as keys.
	// Secondarily, (docker login) accepted URLs with no normalization for
	// several years, and matched registry hostnames against that, so support
	// those entries even in non-legacyFormat ~/.docker/config.json.
	// The docker.io registry still uses the /v1/ key with a special host name,
	// so account for that as well.
	registry = normalizeRegistry(registry)
	for k, v := range auths.AuthConfigs {
		if normalizeAuthFileKey(k, legacyFormat) == registry {
			return decodeDockerAuth(v)
		}
	}

	// Only log this if we found nothing; getCredentialsWithHomeDir logs the
	// source of found data.
	logrus.Debugf("No credentials matching %s found in %s", key, path)
	return types.DockerAuthConfig{}, nil
}

// authKeysForKey returns the keys matching a provided auth file key, in order
// from the best match to worst. For example,
// when given a repository key "quay.io/repo/ns/image", it returns
// - quay.io/repo/ns/image
// - quay.io/repo/ns
// - quay.io/repo
// - quay.io
func authKeysForKey(key string) (res []string) {
	for {
		res = append(res, key)

		lastSlash := strings.LastIndex(key, "/")
		if lastSlash == -1 {
			break
		}
		key = key[:lastSlash]
	}

	return res
}

// decodeDockerAuth decodes the username and password, which is
// encoded in base64.
func decodeDockerAuth(conf dockerAuthConfig) (types.DockerAuthConfig, error) {
	decoded, err := base64.StdEncoding.DecodeString(conf.Auth)
	if err != nil {
		return types.DockerAuthConfig{}, err
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		// if it's invalid just skip, as docker does
		return types.DockerAuthConfig{}, nil
	}

	user := parts[0]
	password := strings.Trim(parts[1], "\x00")
	return types.DockerAuthConfig{
		Username:      user,
		Password:      password,
		IdentityToken: conf.IdentityToken,
	}, nil
}

// normalizeAuthFileKey takes a key, converts it to a host name and normalizes
// the resulting registry.
func normalizeAuthFileKey(key string, legacyFormat bool) string {
	stripped := strings.TrimPrefix(key, "http://")
	stripped = strings.TrimPrefix(stripped, "https://")

	if legacyFormat || stripped != key {
		stripped = strings.SplitN(stripped, "/", 2)[0]
	}

	return normalizeRegistry(stripped)
}

// normalizeRegistry converts the provided registry if a known docker.io host
// is provided.
func normalizeRegistry(registry string) string {
	switch registry {
	case "registry-1.docker.io", "docker.io":
		return "index.docker.io"
	}
	return registry
}

// validateKey verifies that the input key does not have a prefix that is not
// allowed and returns an indicator if the key is namespaced.
func validateKey(key string) (bool, error) {
	if strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://") {
		return false, errors.Errorf("key %s contains http[s]:// prefix", key)
	}

	// Ideally this should only accept explicitly valid keys, compare
	// validateIdentityRemappingPrefix. For now, just reject values that look
	// like tagged or digested values.
	if strings.ContainsRune(key, '@') {
		return false, fmt.Errorf(`key %s contains a '@' character`, key)
	}

	firstSlash := strings.IndexRune(key, '/')
	isNamespaced := firstSlash != -1
	// Reject host/repo:tag, but allow localhost:5000 and localhost:5000/foo.
	if isNamespaced && strings.ContainsRune(key[firstSlash+1:], ':') {
		return false, fmt.Errorf(`key %s contains a ':' character after host[:port]`, key)
	}
	// check if the provided key contains one or more subpaths.
	return isNamespaced, nil
}
