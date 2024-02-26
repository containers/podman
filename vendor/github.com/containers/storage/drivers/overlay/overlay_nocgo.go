//go:build linux && !cgo
// +build linux,!cgo

package overlay

import (
	"fmt"
	"path"

	"github.com/containers/storage/pkg/directory"
)

// ReadWriteDiskUsage returns the disk usage of the writable directory for the ID.
// For Overlay, it attempts to check the XFS quota for size, and falls back to
// finding the size of the "diff" directory.
func (d *Driver) ReadWriteDiskUsage(id string) (*directory.DiskUsage, error) {
	return directory.Usage(path.Join(d.dir(id), "diff"))
}

func getComposeFsHelper() (string, error) {
	return "", fmt.Errorf("composefs not supported on this build")
}

func mountComposefsBlob(dataDir, mountPoint string) error {
	return fmt.Errorf("composefs not supported on this build")
}

func generateComposeFsBlob(verityDigests map[string]string, toc interface{}, composefsDir string) error {
	return fmt.Errorf("composefs not supported on this build")
}
