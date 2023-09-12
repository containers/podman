package volumes

import (
	"os"

	"github.com/containers/buildah/internal/volumes"
)

// CleanCacheMount gets the cache parent created by `--mount=type=cache` and removes it.
func CleanCacheMount() error {
	cacheParent := volumes.CacheParent()
	return os.RemoveAll(cacheParent)
}
