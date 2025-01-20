//go:build amd64 || arm64

package machine

import (
	"path/filepath"
	"testing"

	"github.com/containers/podman/v5/pkg/machine/env"
	"github.com/stretchr/testify/assert"
)

func TestGetSSHIdentityPath(t *testing.T) {
	name := "p-test"
	datadir, err := env.GetGlobalDataDir()
	assert.NoError(t, err)
	identityPath, err := env.GetSSHIdentityPath(name)
	assert.NoError(t, err)
	assert.Equal(t, identityPath, filepath.Join(datadir, name))
}
