package overlay

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/containers/storage/pkg/unshare"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// MountWithOptions creates a subdir of the contentDir based on the source directory
// from the source system.  It then mounts up the source directory on to the
// generated mount point and returns the mount point to the caller.
// But allows api to set custom workdir, upperdir and other overlay options
// Following API is being used by podman at the moment
func MountWithOptions(contentDir, source, dest string, opts *Options) (mount specs.Mount, Err error) {
	mergeDir := filepath.Join(contentDir, "merge")

	// Create overlay mount options for rw/ro.
	var overlayOptions string
	if opts.ReadOnly {
		// Read-only overlay mounts require two lower layer.
		lowerTwo := filepath.Join(contentDir, "lower")
		if err := os.Mkdir(lowerTwo, 0755); err != nil {
			return mount, err
		}
		overlayOptions = fmt.Sprintf("lowerdir=%s:%s,private", escapeColon(source), lowerTwo)
	} else {
		// Read-write overlay mounts want a lower, upper and a work layer.
		workDir := filepath.Join(contentDir, "work")
		upperDir := filepath.Join(contentDir, "upper")

		if opts.WorkDirOptionFragment != "" && opts.UpperDirOptionFragment != "" {
			workDir = opts.WorkDirOptionFragment
			upperDir = opts.UpperDirOptionFragment
		}

		st, err := os.Stat(source)
		if err != nil {
			return mount, err
		}
		if err := os.Chmod(upperDir, st.Mode()); err != nil {
			return mount, err
		}
		if stat, ok := st.Sys().(*syscall.Stat_t); ok {
			if err := os.Chown(upperDir, int(stat.Uid), int(stat.Gid)); err != nil {
				return mount, err
			}
		}
		overlayOptions = fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s,private", escapeColon(source), upperDir, workDir)
	}

	mountProgram := findMountProgram(opts.GraphOpts)
	if mountProgram != "" {
		if err := mountWithMountProgram(mountProgram, overlayOptions, mergeDir); err != nil {
			return mount, err
		}

		mount.Source = mergeDir
		mount.Destination = dest
		mount.Type = "bind"
		mount.Options = []string{"bind", "slave"}
		return mount, nil
	}

	if unshare.IsRootless() {
		/* If a mount_program is not specified, fallback to try mounting native overlay.  */
		overlayOptions = fmt.Sprintf("%s,userxattr", overlayOptions)
	}

	mount.Source = mergeDir
	mount.Destination = dest
	mount.Type = "overlay"
	mount.Options = strings.Split(overlayOptions, ",")

	return mount, nil
}
