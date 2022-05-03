package zfs

import (
	"fmt"

	"github.com/containers/storage/drivers"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func checkRootdirFs(rootdir string) error {
	var buf unix.Statfs_t
	if err := unix.Statfs(rootdir, &buf); err != nil {
		return fmt.Errorf("Failed to access '%s': %s", rootdir, err)
	}

	// on FreeBSD buf.Fstypename contains ['z', 'f', 's', 0 ... ]
	if (buf.Fstypename[0] != 122) || (buf.Fstypename[1] != 102) || (buf.Fstypename[2] != 115) || (buf.Fstypename[3] != 0) {
		logrus.WithField("storage-driver", "zfs").Debugf("no zfs dataset found for rootdir '%s'", rootdir)
		return errors.Wrapf(graphdriver.ErrPrerequisites, "no zfs dataset found for rootdir '%s'", rootdir)
	}

	return nil
}

func getMountpoint(id string) string {
	return id
}

func detachUnmount(mountpoint string) error {
	// FreeBSD's MNT_FORCE is roughly equivalent to MNT_DETACH
	return unix.Unmount(mountpoint, unix.MNT_FORCE)
}
