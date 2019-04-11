package zfs

import (
	"fmt"
	"strings"

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
	maxlen := 12

	// we need to preserve filesystem suffix
	suffix := strings.SplitN(id, "-", 2)

	if len(suffix) > 1 {
		return id[:maxlen] + "-" + suffix[1]
	}

	return id[:maxlen]
}
