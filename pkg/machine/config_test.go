//go:build amd64 || arm64

package machine

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSSHIdentityPath(t *testing.T) {
	name := "p-test"
	datadir, err := GetGlobalDataDir()
	assert.Nil(t, err)
	identityPath, err := GetSSHIdentityPath(name)
	assert.Nil(t, err)
	assert.Equal(t, identityPath, filepath.Join(datadir, name))
}
