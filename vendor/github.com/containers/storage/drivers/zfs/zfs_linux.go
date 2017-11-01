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

	if graphdriver.FsMagic(buf.Type) != graphdriver.FsMagicZfs {
		logrus.Debugf("[zfs] no zfs dataset found for rootdir '%s'", rootdir)
		return errors.Wrapf(graphdriver.ErrPrerequisites, "no zfs dataset found for rootdir '%s'", rootdir)
	}

	return nil
}

func getMountpoint(id string) string {
	return id
}
