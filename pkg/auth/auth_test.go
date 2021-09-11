package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/containers/image/v5/pkg/docker/config"
	"github.com/containers/image/v5/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const largeAuthFile = `{"auths":{
	"docker.io/vendor": {"auth": "ZG9ja2VyOnZlbmRvcg=="},
	"https://index.docker.io/v1": {"auth": "ZG9ja2VyOnRvcA=="},
	"quay.io/libpod": {"auth": "cXVheTpsaWJwb2Q="},
	"quay.io": {"auth": "cXVheTp0b3A="}
}}`

// Semantics of largeAuthFile
var largeAuthFileValues = map[string]types.DockerAuthConfig{
	"docker.io/vendor": {Username: "docker", Password: "vendor"},
	"docker.io":        {Username: "docker", Password: "top"},
	"quay.io/libpod":   {Username: "quay", Password: "libpod"},
	"quay.io":          {Username: "quay", Password: "top"},
}

// Test that GetCredentials() correctly parses what Header() produces
func TestHeaderGetCredentialsRoundtrip(t *testing.T) {
	for _, tc := range []struct {
		headerName         HeaderAuthName
		name               string
		fileContents       string
		username, password string
		expectedOverride   *types.DockerAuthConfig
		expectedFileValues map[string]types.DockerAuthConfig
	}{
		{
			headerName:         XRegistryConfigHeader,
			name:               "no data",
			fileContents:       "",
			username:           "",
			password:           "",
			expectedOverride:   nil,
			expectedFileValues: nil,
		},
		{
			headerName:         XRegistryConfigHeader,
			name:               "file data",
			fileContents:       largeAuthFile,
			username:           "",
			password:           "",
			expectedOverride:   nil,
			expectedFileValues: largeAuthFileValues,
		},
		{
			headerName:         XRegistryConfigHeader,
			name:               "file data + override",
			fileContents:       largeAuthFile,
			username:           "override-user",
			password:           "override-pass",
			expectedOverride:   &types.DockerAuthConfig{Username: "override-user", Password: "override-pass"},
			expectedFileValues: largeAuthFileValues,
		},
		{
			headerName:         XRegistryAuthHeader,
			name:               "override",
			fileContents:       "",
			username:           "override-user",
			password:           "override-pass",
			expectedOverride:   &types.DockerAuthConfig{Username: "override-user", Password: "override-pass"},
			expectedFileValues: nil,
		},
		{
			headerName:         XRegistryAuthHeader,
			name:               "file data",
			fileContents:       largeAuthFile,
			username:           "",
			password:           "",
			expectedFileValues: largeAuthFileValues,
		},
	} {
		name := fmt.Sprintf("%s: %s", tc.headerName, tc.name)
		inputAuthFile := ""
		if tc.fileContents != "" {
			f, err := ioutil.TempFile("", "auth.json")
			require.NoError(t, err, name)
			defer os.Remove(f.Name())
			inputAuthFile = f.Name()
			err = ioutil.WriteFile(inputAuthFile, []byte(tc.fileContents), 0700)
			require.NoError(t, err, name)
		}

		headers, err := Header(nil, tc.headerName, inputAuthFile, tc.username, tc.password)
		require.NoError(t, err)
		req, err := http.NewRequest(http.MethodPost, "/", nil)
		require.NoError(t, err, name)
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		override, resPath, parsedHeader, err := GetCredentials(req)
		require.NoError(t, err, name)
		defer RemoveAuthfile(resPath)
		if tc.expectedOverride == nil {
			assert.Nil(t, override, name)
		} else {
			require.NotNil(t, override, name)
			assert.Equal(t, *tc.expectedOverride, *override, name)
		}
		for key, expectedAuth := range tc.expectedFileValues {
			auth, err := config.GetCredentials(&types.SystemContext{AuthFilePath: resPath}, key)
			require.NoError(t, err, name)
			assert.Equal(t, expectedAuth, auth, "%s, key %s", name, key)
		}
		if len(headers) != 0 {
			assert.Len(t, headers, 1)
			assert.Equal(t, tc.headerName, parsedHeader)
		} else {
			assert.Equal(t, HeaderAuthName(""), parsedHeader)
		}
	}
}

func TestHeader(t *testing.T) {
	for _, tc := range []struct {
		headerName         HeaderAuthName
		name               string
		fileContents       string
		username, password string
		shouldErr          bool
		expectedContents   string
	}{
		{
			headerName:       XRegistryConfigHeader,
			name:             "no data",
			fileContents:     "",
			username:         "",
			password:         "",
			expectedContents: "",
		},
		{
			headerName:   XRegistryConfigHeader,
			name:         "invalid JSON",
			fileContents: "@invalid JSON",
			username:     "",
			password:     "",
			shouldErr:    true,
		},
		{
			headerName:   XRegistryConfigHeader,
			name:         "file data",
			fileContents: largeAuthFile,
			username:     "",
			password:     "",
			expectedContents: `{
			"docker.io/vendor": {"username": "docker", "password": "vendor"},
			"docker.io": {"username": "docker", "password": "top"},
			"quay.io/libpod": {"username": "quay", "password": "libpod"},
			"quay.io": {"username": "quay", "password": "top"}
			}`,
		},
		{
			headerName:   XRegistryConfigHeader,
			name:         "file data + override",
			fileContents: largeAuthFile,
			username:     "override-user",
			password:     "override-pass",
			expectedContents: `{
				"docker.io/vendor": {"username": "docker", "password": "vendor"},
				"docker.io": {"username": "docker", "password": "top"},
				"quay.io/libpod": {"username": "quay", "password": "libpod"},
				"quay.io": {"username": "quay", "password": "top"},
				"": {"username": "override-user", "password": "override-pass"}
				}`,
		},
		{
			headerName:       XRegistryAuthHeader,
			name:             "override",
			fileContents:     "",
			username:         "override-user",
			password:         "override-pass",
			expectedContents: `{"username": "override-user", "password": "override-pass"}`,
		},
		{
			headerName:   XRegistryAuthHeader,
			name:         "invalid JSON",
			fileContents: "@invalid JSON",
			username:     "",
			password:     "",
			shouldErr:    true,
		},
		{
			headerName:   XRegistryAuthHeader,
			name:         "file data",
			fileContents: largeAuthFile,
			username:     "",
			password:     "",
			expectedContents: `{
			"docker.io/vendor": {"username": "docker", "password": "vendor"},
			"docker.io": {"username": "docker", "password": "top"},
			"quay.io/libpod": {"username": "quay", "password": "libpod"},
			"quay.io": {"username": "quay", "password": "top"}
			}`,
		},
	} {
		name := fmt.Sprintf("%s: %s", tc.headerName, tc.name)
		authFile := ""
		if tc.fileContents != "" {
			f, err := ioutil.TempFile("", "auth.json")
			require.NoError(t, err, name)
			defer os.Remove(f.Name())
			authFile = f.Name()
			err = ioutil.WriteFile(authFile, []byte(tc.fileContents), 0700)
			require.NoError(t, err, name)
		}

		res, err := Header(nil, tc.headerName, authFile, tc.username, tc.password)
		if tc.shouldErr {
			assert.Error(t, err, name)
		} else {
			require.NoError(t, err, name)
			if tc.expectedContents == "" {
				assert.Empty(t, res, name)
			} else {
				require.Len(t, res, 1, name)
				header, ok := res[tc.headerName.String()]
				require.True(t, ok, name)
				decodedHeader, err := base64.URLEncoding.DecodeString(header)
				require.NoError(t, err, name)
				// Don't test for a specific JSON representation, just for the expected contents.
				expected := map[string]interface{}{}
				actual := map[string]interface{}{}
				err = json.Unmarshal([]byte(tc.expectedContents), &expected)
				require.NoError(t, err, name)
				err = json.Unmarshal(decodedHeader, &actual)
				require.NoError(t, err, name)
				assert.Equal(t, expected, actual, name)
			}
		}
	}
}

func TestAuthConfigsToAuthFile(t *testing.T) {
	for _, tc := range []struct {
		name             string
		server           string
		shouldErr        bool
		expectedContains string
	}{
		{
			name:             "empty auth configs",
			server:           "",
			shouldErr:        false,
			expectedContains: "{}",
		},
		{
			name:             "registry with a namespace prefix",
			server:           "my-registry.local/username",
			shouldErr:        false,
			expectedContains: `"my-registry.local/username":`,
		},
		{
			name:             "URLs are interpreted as full registries",
			server:           "http://my-registry.local/username",
			shouldErr:        false,
			expectedContains: `"my-registry.local":`,
		},
		{
			name:             "the old-style docker registry URL is normalized",
			server:           "http://index.docker.io/v1/",
			shouldErr:        false,
			expectedContains: `"docker.io":`,
		},
		{
			name:             "docker.io vendor namespace",
			server:           "docker.io/vendor",
			shouldErr:        false,
			expectedContains: `"docker.io/vendor":`,
		},
	} {
		configs := map[string]types.DockerAuthConfig{}
		if tc.server != "" {
			configs[tc.server] = types.DockerAuthConfig{}
		}

		filePath, err := authConfigsToAuthFile(configs)

		if tc.shouldErr {
			assert.Error(t, err)
			assert.Empty(t, filePath)
		} else {
			assert.NoError(t, err)
			content, err := ioutil.ReadFile(filePath)
			require.NoError(t, err)
			assert.Contains(t, string(content), tc.expectedContains)
			os.Remove(filePath)
		}
	}
}

func TestParseSingleAuthHeader(t *testing.T) {
	for _, tc := range []struct {
		input     string
		shouldErr bool
		expected  map[string]types.DockerAuthConfig
	}{
		{
			input:    "", // An empty (or missing) header
			expected: map[string]types.DockerAuthConfig{"0": {}},
		},
		{
			input:    "null",
			expected: map[string]types.DockerAuthConfig{"0": {}},
		},
		// Invalid JSON
		{input: "@", shouldErr: true},
		// Success
		{
			input: base64.URLEncoding.EncodeToString([]byte(`{"username":"u1","password":"p1"}`)),
			expected: map[string]types.DockerAuthConfig{
				"0": {Username: "u1", Password: "p1"},
			},
		},
	} {
		req, err := http.NewRequest(http.MethodPost, "/", nil)
		require.NoError(t, err, tc.input)
		req.Header.Set(XRegistryAuthHeader.String(), tc.input)
		res, err := parseSingleAuthHeader(req)
		if tc.shouldErr {
			assert.Error(t, err, tc.input)
		} else {
			require.NoError(t, err, tc.input)
			assert.Equal(t, tc.expected, res, tc.input)
		}
	}
}

func TestParseMultiAuthHeader(t *testing.T) {
	for _, tc := range []struct {
		input     string
		shouldErr bool
		expected  map[string]types.DockerAuthConfig
	}{
		// Empty header
		{input: "", expected: nil},
		// "null"
		{input: "null", expected: nil},
		// Invalid JSON
		{input: "@", shouldErr: true},
		// Success
		{
			input: base64.URLEncoding.EncodeToString([]byte(
				`{"https://index.docker.io/v1/":{"username":"u1","password":"p1"},` +
					`"quay.io/libpod":{"username":"u2","password":"p2"}}`)),
			expected: map[string]types.DockerAuthConfig{
				"https://index.docker.io/v1/": {Username: "u1", Password: "p1"},
				"quay.io/libpod":              {Username: "u2", Password: "p2"},
			},
		},
	} {
		req, err := http.NewRequest(http.MethodPost, "/", nil)
		require.NoError(t, err, tc.input)
		req.Header.Set(XRegistryAuthHeader.String(), tc.input)
		res, err := parseMultiAuthHeader(req)
		if tc.shouldErr {
			assert.Error(t, err, tc.input)
		} else {
			require.NoError(t, err, tc.input)
			assert.Equal(t, tc.expected, res, tc.input)
		}
	}
}
