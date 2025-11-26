//go:build darwin

package specgen

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveVolumeSourcePathTmpSymlink(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "podman-vol-")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})

	resolved := ResolveVolumeSourcePath(dir)
	require.NotEmpty(t, resolved)
	assert.True(t, strings.HasPrefix(resolved, "/private/tmp/"), "resolved path should point at /private/tmp")
}
