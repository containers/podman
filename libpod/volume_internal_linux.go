// +build linux

package libpod

import (
	"os/exec"
	"strings"

	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// mount mounts the volume if necessary.
// A mount is necessary if a volume has any options set.
// If a mount is necessary, v.state.MountCount will be incremented.
// If it was 0 when the increment occurred, the volume will be mounted on the
// host. Otherwise, we assume it is already mounted.
// Must be done while the volume is locked.
// Is a no-op on volumes that do not require a mount (as defined by
// volumeNeedsMount())
func (v *Volume) mount() error {
	if !v.needsMount() {
		return nil
	}

	// We cannot mount volumes as rootless.
	if rootless.IsRootless() {
		return errors.Wrapf(define.ErrRootless, "cannot mount volumes without root privileges")
	}

	// Update the volume from the DB to get an accurate mount counter.
	if err := v.update(); err != nil {
		return err
	}

	// If the count is non-zero, the volume is already mounted.
	// Nothing to do.
	if v.state.MountCount > 0 {
		v.state.MountCount += 1
		logrus.Debugf("Volume %s mount count now at %d", v.Name(), v.state.MountCount)
		return v.save()
	}

	volDevice := v.config.Options["device"]
	volType := v.config.Options["type"]
	volOptions := v.config.Options["o"]

	// Some filesystems (tmpfs) don't have a device, but we still need to
	// give the kernel something.
	if volDevice == "" && volType != "" {
		volDevice = volType
	}

	// We need to use the actual mount command.
	// Convincing unix.Mount to use the same semantics as the mount command
	// itself seems prohibitively difficult.
	// TODO: might want to cache this path in the runtime?
	mountPath, err := exec.LookPath("mount")
	if err != nil {
		return errors.Wrapf(err, "error locating 'mount' binary")
	}
	mountArgs := []string{}
	if volOptions != "" {
		mountArgs = append(mountArgs, "-o", volOptions)
	}
	if volType != "" {
		mountArgs = append(mountArgs, "-t", volType)
	}
	mountArgs = append(mountArgs, volDevice, v.config.MountPoint)
	mountCmd := exec.Command(mountPath, mountArgs...)

	logrus.Debugf("Running mount command: %s %s", mountPath, strings.Join(mountArgs, " "))
	if output, err := mountCmd.CombinedOutput(); err != nil {
		logrus.Debugf("Mount failed with %v", err)
		return errors.Wrapf(errors.Errorf(string(output)), "error mounting volume %s", v.Name())
	}

	logrus.Debugf("Mounted volume %s", v.Name())

	// Increment the mount counter
	v.state.MountCount += 1
	logrus.Debugf("Volume %s mount count now at %d", v.Name(), v.state.MountCount)
	return v.save()
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
	if !v.needsMount() {
		return nil
	}

	// Update the volume from the DB to get an accurate mount counter.
	if err := v.update(); err != nil {
		return err
	}

	if v.state.MountCount == 0 {
		logrus.Debugf("Volume %s already unmounted", v.Name())
		return nil
	}

	// We cannot unmount volumes as rootless.
	if rootless.IsRootless() {
		// If force is set, just clear the counter and bail without
		// error, so we can remove volumes from the state if they are in
		// an awkward configuration.
		if force {
			logrus.Errorf("Volume %s is mounted despite being rootless - state is not sane", v.Name())
			v.state.MountCount = 0
			return v.save()
		}

		return errors.Wrapf(define.ErrRootless, "cannot mount or unmount volumes without root privileges")
	}

	if !force {
		v.state.MountCount -= 1
	} else {
		v.state.MountCount = 0
	}

	logrus.Debugf("Volume %s mount count now at %d", v.Name(), v.state.MountCount)

	if v.state.MountCount == 0 {
		// Unmount the volume
		if err := unix.Unmount(v.config.MountPoint, unix.MNT_DETACH); err != nil {
			if err == unix.EINVAL {
				// Ignore EINVAL - the mount no longer exists.
				return nil
			}
			return errors.Wrapf(err, "error unmounting volume %s", v.Name())
		}
		logrus.Debugf("Unmounted volume %s", v.Name())
	}

	return v.save()
}
