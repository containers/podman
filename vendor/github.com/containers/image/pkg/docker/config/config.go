package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containers/image/types"
	helperclient "github.com/docker/docker-credential-helpers/client"
	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/docker/docker/pkg/homedir"
	"github.com/pkg/errors"
)

type dockerAuthConfig struct {
	Auth string `json:"auth,omitempty"`
}

type dockerConfigFile struct {
	AuthConfigs map[string]dockerAuthConfig `json:"auths"`
	CredHelpers map[string]string           `json:"credHelpers,omitempty"`
}

const (
	defaultPath       = "/run/user"
	authCfg           = "containers"
	authCfgFileName   = "auth.json"
	dockerCfg         = ".docker"
	dockerCfgFileName = "config.json"
	dockerLegacyCfg   = ".dockercfg"
)

var (
	// ErrNotLoggedIn is returned for users not logged into a registry
	// that they are trying to logout of
	ErrNotLoggedIn = errors.New("not logged in")
)

// SetAuthentication stores the username and password in the auth.json file
func SetAuthentication(ctx *types.SystemContext, registry, username, password string) error {
	return modifyJSON(ctx, func(auths *dockerConfigFile) (bool, error) {
		if ch, exists := auths.CredHelpers[registry]; exists {
			return false, setAuthToCredHelper(ch, registry, username, password)
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
func GetAuthentication(ctx *types.SystemContext, registry string) (string, string, error) {
	if ctx != nil && ctx.DockerAuthConfig != nil {
		return ctx.DockerAuthConfig.Username, ctx.DockerAuthConfig.Password, nil
	}

	dockerLegacyPath := filepath.Join(homedir.Get(), dockerLegacyCfg)
	paths := [3]string{getPathToAuth(ctx), filepath.Join(homedir.Get(), dockerCfg, dockerCfgFileName), dockerLegacyPath}

	for _, path := range paths {
		legacyFormat := path == dockerLegacyPath
		username, password, err := findAuthentication(registry, path, legacyFormat)
		if err != nil {
			return "", "", err
		}
		if username != "" && password != "" {
			return username, password, nil
		}
	}
	return "", "", nil
}

// GetUserLoggedIn returns the username logged in to registry from either
// auth.json or XDG_RUNTIME_DIR
// Used to tell the user if someone is logged in to the registry when logging in
func GetUserLoggedIn(ctx *types.SystemContext, registry string) string {
	path := getPathToAuth(ctx)
	username, _, _ := findAuthentication(registry, path, false)
	if username != "" {
		return username
	}
	return ""
}

// RemoveAuthentication deletes the credentials stored in auth.json
func RemoveAuthentication(ctx *types.SystemContext, registry string) error {
	return modifyJSON(ctx, func(auths *dockerConfigFile) (bool, error) {
		// First try cred helpers.
		if ch, exists := auths.CredHelpers[registry]; exists {
			return false, deleteAuthFromCredHelper(ch, registry)
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
func RemoveAllAuthentication(ctx *types.SystemContext) error {
	return modifyJSON(ctx, func(auths *dockerConfigFile) (bool, error) {
		auths.CredHelpers = make(map[string]string)
		auths.AuthConfigs = make(map[string]dockerAuthConfig)
		return true, nil
	})
}

// getPath gets the path of the auth.json file
// The path can be overriden by the user if the overwrite-path flag is set
// If the flag is not set and XDG_RUNTIME_DIR is ser, the auth.json file is saved in XDG_RUNTIME_DIR/containers
// Otherwise, the auth.json file is stored in /run/user/UID/containers
func getPathToAuth(ctx *types.SystemContext) string {
	if ctx != nil {
		if ctx.AuthFilePath != "" {
			return ctx.AuthFilePath
		}
		if ctx.RootForImplicitAbsolutePaths != "" {
			return filepath.Join(ctx.RootForImplicitAbsolutePaths, defaultPath, strconv.Itoa(os.Getuid()), authCfg, authCfgFileName)
		}
	}
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = filepath.Join(defaultPath, strconv.Itoa(os.Getuid()))
	}
	return filepath.Join(runtimeDir, authCfg, authCfgFileName)
}

// readJSONFile unmarshals the authentications stored in the auth.json file and returns it
// or returns an empty dockerConfigFile data structure if auth.json does not exist
// if the file exists and is empty, readJSONFile returns an error
func readJSONFile(path string, legacyFormat bool) (dockerConfigFile, error) {
	var auths dockerConfigFile

	raw, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		auths.AuthConfigs = map[string]dockerAuthConfig{}
		return auths, nil
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
func modifyJSON(ctx *types.SystemContext, editor func(auths *dockerConfigFile) (bool, error)) error {
	path := getPathToAuth(ctx)
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err = os.Mkdir(dir, 0700); err != nil {
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
