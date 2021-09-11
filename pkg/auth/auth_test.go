package auth

import (
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/containers/image/v5/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			name:             "registry with prefix",
			server:           "my-registry.local/username",
			shouldErr:        false,
			expectedContains: `"my-registry.local/username":`,
		},
		{
			name:             "normalize https:// prefix",
			server:           "http://my-registry.local/username",
			shouldErr:        false,
			expectedContains: `"my-registry.local/username":`,
		},
		{
			name:             "normalize docker registry with https prefix",
			server:           "http://index.docker.io/v1/",
			shouldErr:        false,
			expectedContains: `"index.docker.io":`,
		},
		{
			name:             "normalize docker registry without https prefix",
			server:           "docker.io/v2/",
			shouldErr:        false,
			expectedContains: `"docker.io":`,
		},
	} {
		configs := map[string]types.DockerAuthConfig{}
		if tc.server != "" {
			configs[tc.server] = types.DockerAuthConfig{}
		}

		filePath, err := authConfigsToAuthFile(configs)

		if tc.shouldErr {
			assert.NotNil(t, err)
			assert.Empty(t, filePath)
		} else {
			assert.Nil(t, err)
			content, err := ioutil.ReadFile(filePath)
			assert.Nil(t, err)
			assert.Contains(t, string(content), tc.expectedContains)
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
