//go:build !linux && !freebsd
// +build !linux,!freebsd

package libpod

import (
	"errors"
)

// mount mounts the volume if necessary.
// A mount is necessary if a volume has any options set.
// If a mount is necessary, v.state.MountCount will be incremented.
// If it was 0 when the increment occurred, the volume will be mounted on the
// host. Otherwise, we assume it is already mounted.
// Must be done while the volume is locked.
// Is a no-op on volumes that do not require a mount (as defined by
// volumeNeedsMount()).
func (v *Volume) mount() error {
	return errors.New("not implemented (*Volume) mount")
}

// unmount unmounts the volume if necessary.
// Unmounting a volume that is not mounted is a no-op.
// Unmounting a volume that does not require a mount is a no-op.
// The volume must be locked for this to occur.
// The mount counter will be decremented if non-zero. If the counter reaches 0,
// the volume will really be unmounted, as no further containers are using the
// volume.
// If force is set, the volume will be unmounted regardless of mount counter.
func (v *Volume) unmount(force bool) error {
	return errors.New("not implemented (*Volume) unmount")
}
