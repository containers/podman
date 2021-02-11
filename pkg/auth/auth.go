package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
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

type HeaderAuthName string

func (h HeaderAuthName) String() string { return string(h) }

// XRegistryAuthHeader is the key to the encoded registry authentication configuration in an http-request header.
// This header supports one registry per header occurrence. To support N registries provided N headers, one per registry.
// As of Docker API 1.40 and Libpod API 1.0.0, this header is supported by all endpoints.
const XRegistryAuthHeader HeaderAuthName = "X-Registry-Auth"

// XRegistryConfigHeader is the key to the encoded registry authentication configuration in an http-request header.
// This header supports N registries in one header via a Base64 encoded, JSON map.
// As of Docker API 1.40 and Libpod API 2.0.0, this header is supported by build endpoints.
const XRegistryConfigHeader HeaderAuthName = "X-Registry-Config"

// GetCredentials queries the http.Request for X-Registry-.* headers and extracts
// the necessary authentication information for libpod operations
func GetCredentials(r *http.Request) (*types.DockerAuthConfig, string, HeaderAuthName, error) {
	has := func(key HeaderAuthName) bool { hdr, found := r.Header[string(key)]; return found && len(hdr) > 0 }
	switch {
	case has(XRegistryConfigHeader):
		c, f, err := getConfigCredentials(r)
		return c, f, XRegistryConfigHeader, err
	case has(XRegistryAuthHeader):
		c, f, err := getAuthCredentials(r)
		return c, f, XRegistryAuthHeader, err
	}
	return nil, "", "", nil
}

// getConfigCredentials extracts one or more docker.AuthConfig from the request's
// header.  An empty key will be used as default while a named registry will be
// returned as types.DockerAuthConfig
func getConfigCredentials(r *http.Request) (*types.DockerAuthConfig, string, error) {
	var auth *types.DockerAuthConfig
	configs := make(map[string]types.DockerAuthConfig)

	for _, h := range r.Header[string(XRegistryConfigHeader)] {
		param, err := base64.URLEncoding.DecodeString(h)
		if err != nil {
			return nil, "", errors.Wrapf(err, "failed to decode %q", XRegistryConfigHeader)
		}

		ac := make(map[string]dockerAPITypes.AuthConfig)
		err = json.Unmarshal(param, &ac)
		if err != nil {
			return nil, "", errors.Wrapf(err, "failed to unmarshal %q", XRegistryConfigHeader)
		}

		for k, v := range ac {
			configs[k] = dockerAuthToImageAuth(v)
		}
	}

	// Empty key implies no registry given in API
	if c, found := configs[""]; found {
		auth = &c
	}

	// Override any default given above if specialized credentials provided
	if registries, found := r.URL.Query()["registry"]; found {
		for _, r := range registries {
			for k, v := range configs {
				if strings.Contains(k, r) {
					v := v
					auth = &v
					break
				}
			}
			if auth != nil {
				break
			}
		}

		if auth == nil {
			logrus.Debugf("%q header found in request, but \"registry=%v\" query parameter not provided",
				XRegistryConfigHeader, registries)
		} else {
			logrus.Debugf("%q header found in request for username %q", XRegistryConfigHeader, auth.Username)
		}
	}

	authfile, err := authConfigsToAuthFile(configs)
	return auth, authfile, err
}

// getAuthCredentials extracts one or more DockerAuthConfigs from the request's
// header.  The header could specify a single-auth config in which case the
// first return value is set.  In case of a multi-auth header, the contents are
// stored in a temporary auth file (2nd return value).  Note that the auth file
// should be removed after usage.
func getAuthCredentials(r *http.Request) (*types.DockerAuthConfig, string, error) {
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

// Header builds the requested Authentication Header
func Header(sys *types.SystemContext, headerName HeaderAuthName, authfile, username, password string) (map[string]string, error) {
	var (
		content string
		err     error
	)
	switch headerName {
	case XRegistryAuthHeader:
		content, err = headerAuth(sys, authfile, username, password)
	case XRegistryConfigHeader:
		content, err = headerConfig(sys, authfile, username, password)
	default:
		err = fmt.Errorf("unsupported authentication header: %q", headerName)
	}
	if err != nil {
		return nil, err
	}

	if len(content) > 0 {
		return map[string]string{string(headerName): content}, nil
	}
	return nil, nil
}

// headerConfig returns a map with the XRegistryConfigHeader set which can
// conveniently be used in the http stack.
func headerConfig(sys *types.SystemContext, authfile, username, password string) (string, error) {
	if sys == nil {
		sys = &types.SystemContext{}
	}
	if authfile != "" {
		sys.AuthFilePath = authfile
	}
	authConfigs, err := imageAuth.GetAllCredentials(sys)
	if err != nil {
		return "", err
	}

	if username != "" {
		authConfigs[""] = types.DockerAuthConfig{
			Username: username,
			Password: password,
		}
	}

	if len(authConfigs) == 0 {
		return "", nil
	}
	return encodeMultiAuthConfigs(authConfigs)
}

// headerAuth returns a base64 encoded map with the XRegistryAuthHeader set which can
// conveniently be used in the http stack.
func headerAuth(sys *types.SystemContext, authfile, username, password string) (string, error) {
	if username != "" {
		return encodeSingleAuthConfig(types.DockerAuthConfig{Username: username, Password: password})
	}

	if sys == nil {
		sys = &types.SystemContext{}
	}
	if authfile != "" {
		sys.AuthFilePath = authfile
	}
	authConfigs, err := imageAuth.GetAllCredentials(sys)
	if err != nil {
		return "", err
	}
	return encodeMultiAuthConfigs(authConfigs)
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
	// Initialize an empty temporary JSON file.
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
	authHeader := r.Header.Get(string(XRegistryAuthHeader))
	authConfig := dockerAPITypes.AuthConfig{}
	// Accept "null" and handle it as empty value for compatibility reason with Docker.
	// Some java docker clients pass this value, e.g. this one used in Eclipse.
	if len(authHeader) > 0 && authHeader != "null" {
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
	authHeader := r.Header.Get(string(XRegistryAuthHeader))
	// Accept "null" and handle it as empty value for compatibility reason with Docker.
	// Some java docker clients pass this value, e.g. this one used in Eclipse.
	if len(authHeader) == 0 || authHeader == "null" {
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
