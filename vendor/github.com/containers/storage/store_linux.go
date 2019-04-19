package storage

import (
	"os"

	"github.com/containers/storage/drivers"
	"github.com/pkg/errors"
)

func isOnVolatileStorage(target string) (bool, error) {
	magic, err := graphdriver.GetFSMagic(target)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}
	return magic == graphdriver.FsMagicTmpFs, nil
}

func validateRunRoot(runRoot string) error {
	if onTmpfs, err := isOnVolatileStorage(runRoot); err != nil || !onTmpfs {
		if err != nil {
			return errors.Wrapf(err, "cannot check if %s is on tmpfs", runRoot)
		}
		return errors.Wrapf(ErrTargetNotVolatile, "%s must be on tmpfs", runRoot)
	}
	return nil
}
