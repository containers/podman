package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/image/types"
	helperclient "github.com/docker/docker-credential-helpers/client"
	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/docker/docker/pkg/homedir"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type dockerAuthConfig struct {
	Auth string `json:"auth,omitempty"`
}

type dockerConfigFile struct {
	AuthConfigs map[string]dockerAuthConfig `json:"auths"`
	CredHelpers map[string]string           `json:"credHelpers,omitempty"`
}

var (
	defaultPerUIDPathFormat = filepath.FromSlash("/run/containers/%d/auth.json")
	xdgRuntimeDirPath       = filepath.FromSlash("containers/auth.json")
	dockerHomePath          = filepath.FromSlash(".docker/config.json")
	dockerLegacyHomePath    = ".dockercfg"

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

// GetAuthentication returns the registry credentials stored in
// either auth.json file or .docker/config.json
// If an entry is not found empty strings are returned for the username and password
func GetAuthentication(sys *types.SystemContext, registry string) (string, string, error) {
	if sys != nil && sys.DockerAuthConfig != nil {
		logrus.Debug("Returning credentials from DockerAuthConfig")
		return sys.DockerAuthConfig.Username, sys.DockerAuthConfig.Password, nil
	}

	if enableKeyring {
		username, password, err := getAuthFromKernelKeyring(registry)
		if err == nil {
			logrus.Debug("returning credentials from kernel keyring")
			return username, password, nil
		}
	}

	dockerLegacyPath := filepath.Join(homedir.Get(), dockerLegacyHomePath)
	var paths []string
	pathToAuth, err := getPathToAuth(sys)
	if err == nil {
		paths = append(paths, pathToAuth)
	} else {
		// Error means that the path set for XDG_RUNTIME_DIR does not exist
		// but we don't want to completely fail in the case that the user is pulling a public image
		// Logging the error as a warning instead and moving on to pulling the image
		logrus.Warnf("%v: Trying to pull image in the event that it is a public image.", err)
	}
	paths = append(paths, filepath.Join(homedir.Get(), dockerHomePath), dockerLegacyPath)

	for _, path := range paths {
		legacyFormat := path == dockerLegacyPath
		username, password, err := findAuthentication(registry, path, legacyFormat)
		if err != nil {
			logrus.Debugf("Credentials not found")
			return "", "", err
		}
		if username != "" && password != "" {
			logrus.Debugf("Returning credentials from %s", path)
			return username, password, nil
		}
	}
	logrus.Debugf("Credentials not found")
	return "", "", nil
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

// RemoveAllAuthentication deletes all the credentials stored in auth.json
func RemoveAllAuthentication(sys *types.SystemContext) error {
	return modifyJSON(sys, func(auths *dockerConfigFile) (bool, error) {
		auths.CredHelpers = make(map[string]string)
		auths.AuthConfigs = make(map[string]dockerAuthConfig)
		return true, nil
	})
}

// getPath gets the path of the auth.json file
// The path can be overriden by the user if the overwrite-path flag is set
// If the flag is not set and XDG_RUNTIME_DIR is set, the auth.json file is saved in XDG_RUNTIME_DIR/containers
// Otherwise, the auth.json file is stored in /run/containers/UID
func getPathToAuth(sys *types.SystemContext) (string, error) {
	if sys != nil {
		if sys.AuthFilePath != "" {
			return sys.AuthFilePath, nil
		}
		if sys.RootForImplicitAbsolutePaths != "" {
			return filepath.Join(sys.RootForImplicitAbsolutePaths, fmt.Sprintf(defaultPerUIDPathFormat, os.Getuid())), nil
		}
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
			return "", errors.Wrapf(err, "%q directory set by $XDG_RUNTIME_DIR does not exist. Either create the directory or unset $XDG_RUNTIME_DIR.", runtimeDir)
		} // else ignore err and let the caller fail accessing xdgRuntimeDirPath.
		return filepath.Join(runtimeDir, xdgRuntimeDirPath), nil
	}
	return fmt.Sprintf(defaultPerUIDPathFormat, os.Getuid()), nil
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

	return auths, nil
}

// modifyJSON writes to auth.json if the dockerConfigFile has been updated
func modifyJSON(sys *types.SystemContext, editor func(auths *dockerConfigFile) (bool, error)) error {
	path, err := getPathToAuth(sys)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err = os.MkdirAll(dir, 0700); err != nil {
			return errors.Wrapf(err, "error creating directory %q", dir)
		}
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
func findAuthentication(registry, path string, legacyFormat bool) (string, string, error) {
	auths, err := readJSONFile(path, legacyFormat)
	if err != nil {
		return "", "", errors.Wrapf(err, "error reading JSON file %q", path)
	}

	// First try cred helpers. They should always be normalized.
	if ch, exists := auths.CredHelpers[registry]; exists {
		return getAuthFromCredHelper(ch, registry)
	}

	// I'm feeling lucky
	if val, exists := auths.AuthConfigs[registry]; exists {
		return decodeDockerAuth(val.Auth)
	}

	// bad luck; let's normalize the entries first
	registry = normalizeRegistry(registry)
	normalizedAuths := map[string]dockerAuthConfig{}
	for k, v := range auths.AuthConfigs {
		normalizedAuths[normalizeRegistry(k)] = v
	}
	if val, exists := normalizedAuths[registry]; exists {
		return decodeDockerAuth(val.Auth)
	}
	return "", "", nil
}

func decodeDockerAuth(s string) (string, string, error) {
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", "", err
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		// if it's invalid just skip, as docker does
		return "", "", nil
	}
	user := parts[0]
	password := strings.Trim(parts[1], "\x00")
	return user, password, nil
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
