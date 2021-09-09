package auth

import (
	"io/ioutil"
	"testing"

	"github.com/containers/image/v5/types"
	"github.com/stretchr/testify/assert"
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
