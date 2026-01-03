package shortnames_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.podman.io/image/v5/pkg/shortnames"
	"go.podman.io/image/v5/pkg/sysregistriesv2"
	"go.podman.io/image/v5/types"
)

func TestSingleNameRegistryBypassesShortNameResolution(t *testing.T) {
	sysregistriesv2.InvalidateCache()
	t.Cleanup(sysregistriesv2.InvalidateCache)

	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "registries.conf")
	conf := `
unqualified-search-registries=["docker.io"]
single-name-registries=["registry"]
`
	require.NoError(t, os.WriteFile(confPath, []byte(conf), 0o644))

	ctx := &types.SystemContext{
		SystemRegistriesConfPath: confPath,
	}

	registries, _, err := sysregistriesv2.SingleNameRegistriesWithOrigin(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"registry"}, registries)

	resolved, err := shortnames.Resolve(ctx, "registry/alpine")
	require.NoError(t, err)
	require.Len(t, resolved.PullCandidates, 1)
	require.Equal(t, "registry/alpine:latest", resolved.PullCandidates[0].Value.String())
}
