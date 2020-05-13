package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containers/image/v5/types"
	helperclient "github.com/docker/docker-credential-helpers/client"
	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/docker/docker/pkg/homedir"
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
	xdgRuntimeDirPath       = filepath.FromSlash("containers/auth.json")
	dockerHomePath          = filepath.FromSlash(".docker/config.json")
	dockerLegacyHomePath    = ".dockercfg"
	nonLinuxAuthFilePath    = filepath.FromSlash(".config/containers/auth.json")

	// Note that the keyring support has been disabled as it was causing
	// regressions. Before enabling, please revisit TODO(keyring) comments
	// which need to be addressed if the need remerged to support the
	// kernel keyring.
	enableKeyring = false

	// ErrNotLoggedIn is returned for users not logged into a registry
	// that they are trying to logout of
	ErrNotLoggedIn = errors.New("not logged in")
	// ErrNotSupported is returned for unsupported methods
	ErrNotSupported = errors.New("not supported")
)

// SetAuthentication stores the username and password in the auth.json file
func SetAuthentication(sys *types.SystemContext, registry, username, password string) error {
	return modifyJSON(sys, func(auths *dockerConfigFile) (bool, error) {
		if ch, exists := auths.CredHelpers[registry]; exists {
			return false, setAuthToCredHelper(ch, registry, username, password)
		}

		// Set the credentials to kernel keyring if enableKeyring is true.
		// The keyring might not work in all environments (e.g., missing capability) and isn't supported on all platforms.
		// Hence, we want to fall-back to using the authfile in case the keyring failed.
		// However, if the enableKeyring is false, we want adhere to the user specification and not use the keyring.
		if enableKeyring {
			err := setAuthToKernelKeyring(registry, username, password)
			if err == nil {
				logrus.Debugf("credentials for (%s, %s) were stored in the kernel keyring\n", registry, username)
				return false, nil
			}
			logrus.Debugf("failed to authenticate with the kernel keyring, falling back to authfiles. %v", err)
		}
		creds := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
		newCreds := dockerAuthConfig{Auth: creds}
		auths.AuthConfigs[registry] = newCreds
		return true, nil
	})
}

// GetAllCredentials returns the registry credentials for all registries stored
// in either the auth.json file or the docker/config.json.
func GetAllCredentials(sys *types.SystemContext) (map[string]types.DockerAuthConfig, error) {
	// Note: we need to read the auth files in the inverse order to prevent
	// a priority inversion when writing to the map.
	authConfigs := make(map[string]types.DockerAuthConfig)
	paths := getAuthFilePaths(sys)
	for i := len(paths) - 1; i >= 0; i-- {
		path := paths[i]
		// readJSONFile returns an empty map in case the path doesn't exist.
		auths, err := readJSONFile(path.path, path.legacyFormat)
		if err != nil {
			return nil, errors.Wrapf(err, "error reading JSON file %q", path.path)
		}

		for registry, data := range auths.AuthConfigs {
			conf, err := decodeDockerAuth(data)
			if err != nil {
				return nil, err
			}
			authConfigs[normalizeRegistry(registry)] = conf
		}

		// Credential helpers may override credentials from the auth file.
		for registry, credHelper := range auths.CredHelpers {
			username, password, err := getAuthFromCredHelper(credHelper, registry)
			if err != nil {
				if credentials.IsErrCredentialsNotFoundMessage(err.Error()) {
					continue
				}
				return nil, err
			}

			conf := types.DockerAuthConfig{Username: username, Password: password}
			authConfigs[normalizeRegistry(registry)] = conf
		}
	}

	// TODO(keyring): if we ever reenable the keyring support, we had to
	// query all credentials from the keyring here.

	return authConfigs, nil
}

// getAuthFilePaths returns a slice of authPaths based on the system context
// in the order they should be searched. Note that some paths may not exist.
func getAuthFilePaths(sys *types.SystemContext) []authPath {
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
	paths = append(paths,
		authPath{path: filepath.Join(homedir.Get(), dockerHomePath), legacyFormat: false},
		authPath{path: filepath.Join(homedir.Get(), dockerLegacyHomePath), legacyFormat: true},
	)
	return paths
}

// GetCredentials returns the registry credentials stored in either auth.json
// file or .docker/config.json, including support for OAuth2 and IdentityToken.
// If an entry is not found, an empty struct is returned.
func GetCredentials(sys *types.SystemContext, registry string) (types.DockerAuthConfig, error) {
	if sys != nil && sys.DockerAuthConfig != nil {
		logrus.Debug("Returning credentials from DockerAuthConfig")
		return *sys.DockerAuthConfig, nil
	}

	if enableKeyring {
		username, password, err := getAuthFromKernelKeyring(registry)
		if err == nil {
			logrus.Debug("returning credentials from kernel keyring")
			return types.DockerAuthConfig{
				Username: username,
				Password: password,
			}, nil
		}
	}

	for _, path := range getAuthFilePaths(sys) {
		authConfig, err := findAuthentication(registry, path.path, path.legacyFormat)
		if err != nil {
			logrus.Debugf("Credentials not found")
			return types.DockerAuthConfig{}, err
		}

		if (authConfig.Username != "" && authConfig.Password != "") || authConfig.IdentityToken != "" {
			logrus.Debugf("Returning credentials from %s", path.path)
			return authConfig, nil
		}
	}

	logrus.Debugf("Credentials not found")
	return types.DockerAuthConfig{}, nil
}

// GetAuthentication returns the registry credentials stored in
// either auth.json file or .docker/config.json
// If an entry is not found empty strings are returned for the username and password
//
// Deprecated: This API only has support for username and password. To get the
// support for oauth2 in docker registry authentication, we added the new
// GetCredentials API. The new API should be used and this API is kept to
// maintain backward compatibility.
func GetAuthentication(sys *types.SystemContext, registry string) (string, string, error) {
	auth, err := GetCredentials(sys, registry)
	if err != nil {
		return "", "", err
	}
	if auth.IdentityToken != "" {
		return "", "", errors.Wrap(ErrNotSupported, "non-empty identity token found and this API doesn't support it")
	}
	return auth.Username, auth.Password, nil
}

// RemoveAuthentication deletes the credentials stored in auth.json
func RemoveAuthentication(sys *types.SystemContext, registry string) error {
	return modifyJSON(sys, func(auths *dockerConfigFile) (bool, error) {
		// First try cred helpers.
		if ch, exists := auths.CredHelpers[registry]; exists {
			return false, deleteAuthFromCredHelper(ch, registry)
		}

		// Next if keyring is enabled try kernel keyring
		if enableKeyring {
			err := deleteAuthFromKernelKeyring(registry)
			if err == nil {
				logrus.Debugf("credentials for %s were deleted from the kernel keyring", registry)
				return false, nil
			}
			logrus.Debugf("failed to delete credentials from the kernel keyring, falling back to authfiles")
		}

		if _, ok := auths.AuthConfigs[registry]; ok {
			delete(auths.AuthConfigs, registry)
		} else if _, ok := auths.AuthConfigs[normalizeRegistry(registry)]; ok {
			delete(auths.AuthConfigs, normalizeRegistry(registry))
		} else {
			return false, ErrNotLoggedIn
		}
		return true, nil
	})
}

// RemoveAllAuthentication deletes all the credentials stored in auth.json and kernel keyring
func RemoveAllAuthentication(sys *types.SystemContext) error {
	return modifyJSON(sys, func(auths *dockerConfigFile) (bool, error) {
		if enableKeyring {
			err := removeAllAuthFromKernelKeyring()
			if err == nil {
				logrus.Debugf("removing all credentials from kernel keyring")
				return false, nil
			}
			logrus.Debugf("error removing credentials from kernel keyring")
		}
		auths.CredHelpers = make(map[string]string)
		auths.AuthConfigs = make(map[string]dockerAuthConfig)
		return true, nil
	})
}

// getPathToAuth gets the path of the auth.json file used for reading and writting credentials
// returns the path, and a bool specifies whether the file is in legacy format
func getPathToAuth(sys *types.SystemContext) (string, bool, error) {
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
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
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
			return dockerConfigFile{}, errors.Wrapf(err, "error unmarshaling JSON at %q", path)
		}
		return auths, nil
	}

	if err = json.Unmarshal(raw, &auths); err != nil {
		return dockerConfigFile{}, errors.Wrapf(err, "error unmarshaling JSON at %q", path)
	}

	if auths.AuthConfigs == nil {
		auths.AuthConfigs = map[string]dockerAuthConfig{}
	}
	if auths.CredHelpers == nil {
		auths.CredHelpers = make(map[string]string)
	}

	return auths, nil
}

// modifyJSON writes to auth.json if the dockerConfigFile has been updated
func modifyJSON(sys *types.SystemContext, editor func(auths *dockerConfigFile) (bool, error)) error {
	path, legacyFormat, err := getPathToAuth(sys)
	if err != nil {
		return err
	}
	if legacyFormat {
		return fmt.Errorf("writes to %s using legacy format are not supported", path)
	}

	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	auths, err := readJSONFile(path, false)
	if err != nil {
		return errors.Wrapf(err, "error reading JSON file %q", path)
	}

	updated, err := editor(&auths)
	if err != nil {
		return errors.Wrapf(err, "error updating %q", path)
	}
	if updated {
		newData, err := json.MarshalIndent(auths, "", "\t")
		if err != nil {
			return errors.Wrapf(err, "error marshaling JSON %q", path)
		}

		if err = ioutil.WriteFile(path, newData, 0755); err != nil {
			return errors.Wrapf(err, "error writing to file %q", path)
		}
	}

	return nil
}

func getAuthFromCredHelper(credHelper, registry string) (string, string, error) {
	helperName := fmt.Sprintf("docker-credential-%s", credHelper)
	p := helperclient.NewShellProgramFunc(helperName)
	creds, err := helperclient.Get(p, registry)
	if err != nil {
		return "", "", err
	}
	return creds.Username, creds.Secret, nil
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

// findAuthentication looks for auth of registry in path
func findAuthentication(registry, path string, legacyFormat bool) (types.DockerAuthConfig, error) {
	auths, err := readJSONFile(path, legacyFormat)
	if err != nil {
		return types.DockerAuthConfig{}, errors.Wrapf(err, "error reading JSON file %q", path)
	}

	// First try cred helpers. They should always be normalized.
	if ch, exists := auths.CredHelpers[registry]; exists {
		username, password, err := getAuthFromCredHelper(ch, registry)
		if err != nil {
			return types.DockerAuthConfig{}, err
		}

		return types.DockerAuthConfig{
			Username: username,
			Password: password,
		}, nil
	}

	// I'm feeling lucky
	if val, exists := auths.AuthConfigs[registry]; exists {
		return decodeDockerAuth(val)
	}

	// bad luck; let's normalize the entries first
	registry = normalizeRegistry(registry)
	normalizedAuths := map[string]dockerAuthConfig{}
	for k, v := range auths.AuthConfigs {
		normalizedAuths[normalizeRegistry(k)] = v
	}

	if val, exists := normalizedAuths[registry]; exists {
		return decodeDockerAuth(val)
	}

	return types.DockerAuthConfig{}, nil
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

// convertToHostname converts a registry url which has http|https prepended
// to just an hostname.
// Copied from github.com/docker/docker/registry/auth.go
func convertToHostname(url string) string {
	stripped := url
	if strings.HasPrefix(url, "http://") {
		stripped = strings.TrimPrefix(url, "http://")
	} else if strings.HasPrefix(url, "https://") {
		stripped = strings.TrimPrefix(url, "https://")
	}

	nameParts := strings.SplitN(stripped, "/", 2)

	return nameParts[0]
}

func normalizeRegistry(registry string) string {
	normalized := convertToHostname(registry)
	switch normalized {
	case "registry-1.docker.io", "docker.io":
		return "index.docker.io"
	}
	return normalized
}
