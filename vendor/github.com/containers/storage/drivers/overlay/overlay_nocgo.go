// +build linux,!cgo

package overlay

import (
	"path"

	"github.com/containers/storage/pkg/directory"
)

// ReadWriteDiskUsage returns the disk usage of the writable directory for the ID.
// For Overlay, it attempts to check the XFS quota for size, and falls back to
// finding the size of the "diff" directory.
func (d *Driver) ReadWriteDiskUsage(id string) (*directory.DiskUsage, error) {
	return directory.Usage(path.Join(d.dir(id), "diff"))
}
