package unshare

import (
	"fmt"
	"os"
	"os/user"
	"sync"

	"github.com/pkg/errors"
	"github.com/syndtr/gocapability/capability"
)

var (
	homeDirOnce sync.Once
	homeDirErr  error
	homeDir     string

	hasCapSysAdminOnce sync.Once
	hasCapSysAdminRet  bool
	hasCapSysAdminErr  error
)

// HomeDir returns the home directory for the current user.
func HomeDir() (string, error) {
	homeDirOnce.Do(func() {
		home := os.Getenv("HOME")
		if home == "" {
			usr, err := user.LookupId(fmt.Sprintf("%d", GetRootlessUID()))
			if err != nil {
				homeDir, homeDirErr = "", errors.Wrapf(err, "unable to resolve HOME directory")
				return
			}
			homeDir, homeDirErr = usr.HomeDir, nil
			return
		}
		homeDir, homeDirErr = home, nil
	})
	return homeDir, homeDirErr
}

// HasCapSysAdmin returns whether the current process has CAP_SYS_ADMIN.
func HasCapSysAdmin() (bool, error) {
	hasCapSysAdminOnce.Do(func() {
		currentCaps, err := capability.NewPid2(0)
		if err != nil {
			hasCapSysAdminErr = err
			return
		}
		if err = currentCaps.Load(); err != nil {
			hasCapSysAdminErr = err
			return
		}
		hasCapSysAdminRet = currentCaps.Get(capability.EFFECTIVE, capability.CAP_SYS_ADMIN)
	})
	return hasCapSysAdminRet, hasCapSysAdminErr
}
