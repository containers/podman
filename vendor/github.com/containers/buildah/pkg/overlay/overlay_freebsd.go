package overlay

import (
	//"fmt"
	//"os"
	//"path/filepath"
	//"strings"
	//"syscall"
	"errors"

	//"github.com/containers/storage/pkg/unshare"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// MountWithOptions creates a subdir of the contentDir based on the source directory
// from the source system.  It then mounts up the source directory on to the
// generated mount point and returns the mount point to the caller.
// But allows api to set custom workdir, upperdir and other overlay options
// Following API is being used by podman at the moment
func MountWithOptions(contentDir, source, dest string, opts *Options) (mount specs.Mount, Err error) {
	if opts.ReadOnly {
		// Read-only overlay mounts can be simulated with nullfs
		mount.Source = source
		mount.Destination = dest
		mount.Type = "nullfs"
		mount.Options = []string{"ro"}
		return mount, nil
	} else {
		return mount, errors.New("read/write overlay mounts not supported on freebsd")
	}
}
