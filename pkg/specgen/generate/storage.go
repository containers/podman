package generate

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/containers/libpod/pkg/util"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

// TODO unify this in one place - maybe libpod/define
const (
	// TypeBind is the type for mounting host dir
	TypeBind = "bind"
	// TypeVolume is the type for named volumes
	TypeVolume = "volume"
	// TypeTmpfs is the type for mounting tmpfs
	TypeTmpfs = "tmpfs"
)

// Supersede existing mounts in the spec with new, user-specified mounts.
// TODO: Should we unmount subtree mounts? E.g., if /tmp/ is mounted by
// one mount, and we already have /tmp/a and /tmp/b, should we remove
// the /tmp/a and /tmp/b mounts in favor of the more general /tmp?
func SupercedeUserMounts(mounts []spec.Mount, configMount []spec.Mount) []spec.Mount {
	if len(mounts) > 0 {
		// If we have overlappings mounts, remove them from the spec in favor of
		// the user-added volume mounts
		destinations := make(map[string]bool)
		for _, mount := range mounts {
			destinations[path.Clean(mount.Destination)] = true
		}
		// Copy all mounts from spec to defaultMounts, except for
		//  - mounts overridden by a user supplied mount;
		//  - all mounts under /dev if a user supplied /dev is present;
		mountDev := destinations["/dev"]
		for _, mount := range configMount {
			if _, ok := destinations[path.Clean(mount.Destination)]; !ok {
				if mountDev && strings.HasPrefix(mount.Destination, "/dev/") {
					// filter out everything under /dev if /dev is user-mounted
					continue
				}

				logrus.Debugf("Adding mount %s", mount.Destination)
				mounts = append(mounts, mount)
			}
		}
		return mounts
	}
	return configMount
}

func InitFSMounts(mounts []spec.Mount) error {
	for i, m := range mounts {
		switch {
		case m.Type == TypeBind:
			opts, err := util.ProcessOptions(m.Options, false, m.Source)
			if err != nil {
				return err
			}
			mounts[i].Options = opts
		case m.Type == TypeTmpfs && filepath.Clean(m.Destination) != "/dev":
			opts, err := util.ProcessOptions(m.Options, true, "")
			if err != nil {
				return err
			}
			mounts[i].Options = opts
		}
	}
	return nil
}
