package zfs

import (
	"github.com/containers/storage/drivers"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func checkRootdirFs(rootDir string) error {
	fsMagic, err := graphdriver.GetFSMagic(rootDir)
	if err != nil {
		return err
	}
	backingFS := "unknown"
	if fsName, ok := graphdriver.FsNames[fsMagic]; ok {
		backingFS = fsName
	}

	if fsMagic != graphdriver.FsMagicZfs {
		logrus.WithField("root", rootDir).WithField("backingFS", backingFS).WithField("storage-driver", "zfs").Error("No zfs dataset found for root")
		return errors.Wrapf(graphdriver.ErrPrerequisites, "no zfs dataset found for rootdir '%s'", rootDir)
	}

	return nil
}

func getMountpoint(id string) string {
	return id
}
