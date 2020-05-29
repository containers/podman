package auth

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	imageAuth "github.com/containers/image/v5/pkg/docker/config"
	"github.com/containers/image/v5/types"
	dockerAPITypes "github.com/docker/docker/api/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// XRegistryAuthHeader is the key to the encoded registry authentication
// configuration in an http-request header.
const XRegistryAuthHeader = "X-Registry-Auth"

// GetCredentials extracts one or more DockerAuthConfigs from the request's
// header.  The header could specify a single-auth config in which case the
// first return value is set.  In case of a multi-auth header, the contents are
// stored in a temporary auth file (2nd return value).  Note that the auth file
// should be removed after usage.
func GetCredentials(r *http.Request) (*types.DockerAuthConfig, string, error) {
	authHeader := r.Header.Get(XRegistryAuthHeader)
	if len(authHeader) == 0 {
		return nil, "", nil
	}

	// First look for a multi-auth header (i.e., a map).
	authConfigs, err := multiAuthHeader(r)
	if err == nil {
		authfile, err := authConfigsToAuthFile(authConfigs)
		return nil, authfile, err
	}

	// Fallback to looking for a single-auth header (i.e., one config).
	authConfigs, err = singleAuthHeader(r)
	if err != nil {
		return nil, "", err
	}
	var conf *types.DockerAuthConfig
	for k := range authConfigs {
		c := authConfigs[k]
		conf = &c
		break
	}
	return conf, "", nil
}

// Header returns a map with the XRegistryAuthHeader set which can
// conveniently be used in the http stack.
func Header(sys *types.SystemContext, authfile, username, password string) (map[string]string, error) {
	var content string
	var err error

	if username != "" {
		content, err = encodeSingleAuthConfig(types.DockerAuthConfig{Username: username, Password: password})
		if err != nil {
			return nil, err
		}
	} else {
		if sys == nil {
			sys = &types.SystemContext{}
		}
		if authfile != "" {
			sys.AuthFilePath = authfile
		}
		authConfigs, err := imageAuth.GetAllCredentials(sys)
		if err != nil {
			return nil, err
		}
		content, err = encodeMultiAuthConfigs(authConfigs)
		if err != nil {
			return nil, err
		}
	}

	header := make(map[string]string)
	header[XRegistryAuthHeader] = content

	return header, nil
}

// RemoveAuthfile is a convenience function that is meant to be called in a
// deferred statement. If non-empty, it removes the specified authfile and log
// errors.  It's meant to reduce boilerplate code at call sites of
// `GetCredentials`.
func RemoveAuthfile(authfile string) {
	if authfile == "" {
		return
	}
	if err := os.Remove(authfile); err != nil {
		logrus.Errorf("Error removing temporary auth file %q: %v", authfile, err)
	}
}

// encodeSingleAuthConfig serializes the auth configuration as a base64 encoded JSON payload.
func encodeSingleAuthConfig(authConfig types.DockerAuthConfig) (string, error) {
	conf := imageAuthToDockerAuth(authConfig)
	buf, err := json.Marshal(conf)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}

// encodeMultiAuthConfigs serializes the auth configurations as a base64 encoded JSON payload.
func encodeMultiAuthConfigs(authConfigs map[string]types.DockerAuthConfig) (string, error) {
	confs := make(map[string]dockerAPITypes.AuthConfig)
	for registry, authConf := range authConfigs {
		confs[registry] = imageAuthToDockerAuth(authConf)
	}
	buf, err := json.Marshal(confs)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}

// authConfigsToAuthFile stores the specified auth configs in a temporary files
// and returns its path. The file can later be used an auth file for contacting
// one or more container registries.  If tmpDir is empty, the system's default
// TMPDIR will be used.
func authConfigsToAuthFile(authConfigs map[string]types.DockerAuthConfig) (string, error) {
	// Intitialize an empty temporary JSON file.
	tmpFile, err := ioutil.TempFile("", "auth.json.")
	if err != nil {
		return "", err
	}
	if _, err := tmpFile.Write([]byte{'{', '}'}); err != nil {
		return "", errors.Wrap(err, "error initializing temporary auth file")
	}
	if err := tmpFile.Close(); err != nil {
		return "", errors.Wrap(err, "error closing temporary auth file")
	}
	authFilePath := tmpFile.Name()

	// TODO: It would be nice if c/image could dump the map at once.
	//
	// Now use the c/image packages to store the credentials. It's battle
	// tested, and we make sure to use the same code as the image backend.
	sys := types.SystemContext{AuthFilePath: authFilePath}
	for server, config := range authConfigs {
		// Note that we do not validate the credentials here. Wassume
		// that all credentials are valid. They'll be used on demand
		// later.
		if err := imageAuth.SetAuthentication(&sys, server, config.Username, config.Password); err != nil {
			return "", errors.Wrapf(err, "error storing credentials in temporary auth file (server: %q, user: %q)", server, config.Username)
		}
	}

	return authFilePath, nil
}

// dockerAuthToImageAuth converts a docker auth config to one we're using
// internally from c/image.  Note that the Docker types look slightly
// different, so we need to convert to be extra sure we're not running into
// undesired side-effects when unmarhalling directly to our types.
func dockerAuthToImageAuth(authConfig dockerAPITypes.AuthConfig) types.DockerAuthConfig {
	return types.DockerAuthConfig{
		Username:      authConfig.Username,
		Password:      authConfig.Password,
		IdentityToken: authConfig.IdentityToken,
	}
}

// reverse conversion of `dockerAuthToImageAuth`.
func imageAuthToDockerAuth(authConfig types.DockerAuthConfig) dockerAPITypes.AuthConfig {
	return dockerAPITypes.AuthConfig{
		Username:      authConfig.Username,
		Password:      authConfig.Password,
		IdentityToken: authConfig.IdentityToken,
	}
}

// singleAuthHeader extracts a DockerAuthConfig from the request's header.
// The header content is a single DockerAuthConfig.
func singleAuthHeader(r *http.Request) (map[string]types.DockerAuthConfig, error) {
	authHeader := r.Header.Get(XRegistryAuthHeader)
	authConfig := dockerAPITypes.AuthConfig{}
	if len(authHeader) > 0 {
		authJSON := base64.NewDecoder(base64.URLEncoding, strings.NewReader(authHeader))
		if err := json.NewDecoder(authJSON).Decode(&authConfig); err != nil {
			return nil, err
		}
	}
	authConfigs := make(map[string]types.DockerAuthConfig)
	authConfigs["0"] = dockerAuthToImageAuth(authConfig)
	return authConfigs, nil
}

// multiAuthHeader extracts a DockerAuthConfig from the request's header.
// The header content is a map[string]DockerAuthConfigs.
func multiAuthHeader(r *http.Request) (map[string]types.DockerAuthConfig, error) {
	authHeader := r.Header.Get(XRegistryAuthHeader)
	if len(authHeader) == 0 {
		return nil, nil
	}

	dockerAuthConfigs := make(map[string]dockerAPITypes.AuthConfig)
	authJSON := base64.NewDecoder(base64.URLEncoding, strings.NewReader(authHeader))
	if err := json.NewDecoder(authJSON).Decode(&dockerAuthConfigs); err != nil {
		return nil, err
	}

	// Now convert to the internal types.
	authConfigs := make(map[string]types.DockerAuthConfig)
	for server := range dockerAuthConfigs {
		authConfigs[server] = dockerAuthToImageAuth(dockerAuthConfigs[server])
	}
	return authConfigs, nil
}
