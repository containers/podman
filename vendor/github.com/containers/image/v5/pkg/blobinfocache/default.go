package blobinfocache

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/containers/image/v5/internal/rootless"
	"github.com/containers/image/v5/pkg/blobinfocache/boltdb"
	"github.com/containers/image/v5/pkg/blobinfocache/memory"
	"github.com/containers/image/v5/types"
	"github.com/sirupsen/logrus"
)

const (
	// blobInfoCacheFilename is the file name used for blob info caches.
	// If the format changes in an incompatible way, increase the version number.
	blobInfoCacheFilename = "blob-info-cache-v1.boltdb"
	// systemBlobInfoCacheDir is the directory containing the blob info cache (in blobInfocacheFilename) for root-running processes.
	systemBlobInfoCacheDir = "/var/lib/containers/cache"
)

// blobInfoCacheDir returns a path to a blob info cache appropriate for sys and euid.
// euid is used so that (sudo …) does not write root-owned files into the unprivileged users’ home directory.
func blobInfoCacheDir(sys *types.SystemContext, euid int) (string, error) {
	if sys != nil && sys.BlobInfoCacheDir != "" {
		return sys.BlobInfoCacheDir, nil
	}

	// FIXME? On Windows, os.Geteuid() returns -1.  What should we do?  Right now we treat it as unprivileged
	// and fail (fall back to memory-only) if neither HOME nor XDG_DATA_HOME is set, which is, at least, safe.
	if euid == 0 {
		if sys != nil && sys.RootForImplicitAbsolutePaths != "" {
			return filepath.Join(sys.RootForImplicitAbsolutePaths, systemBlobInfoCacheDir), nil
		}
		return systemBlobInfoCacheDir, nil
	}

	// This is intended to mirror the GraphRoot determination in github.com/containers/libpod/pkg/util.GetRootlessStorageOpts.
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home := os.Getenv("HOME")
		if home == "" {
			return "", fmt.Errorf("neither XDG_DATA_HOME nor HOME was set non-empty")
		}
		dataDir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataDir, "containers", "cache"), nil
}

// DefaultCache returns the default BlobInfoCache implementation appropriate for sys.
func DefaultCache(sys *types.SystemContext) types.BlobInfoCache {
	dir, err := blobInfoCacheDir(sys, rootless.GetRootlessEUID())
	if err != nil {
		logrus.Debugf("Error determining a location for %s, using a memory-only cache", blobInfoCacheFilename)
		return memory.New()
	}
	path := filepath.Join(dir, blobInfoCacheFilename)
	if err := os.MkdirAll(dir, 0700); err != nil {
		logrus.Debugf("Error creating parent directories for %s, using a memory-only cache: %v", blobInfoCacheFilename, err)
		return memory.New()
	}

	logrus.Debugf("Using blob info cache at %s", path)
	return boltdb.New(path)
}
