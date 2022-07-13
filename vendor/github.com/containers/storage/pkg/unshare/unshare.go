package unshare

import (
	"fmt"
	"os"
	"os/user"
	"sync"

	"github.com/sirupsen/logrus"
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
				homeDir, homeDirErr = "", fmt.Errorf("unable to resolve HOME directory: %w", err)
				return
			}
			homeDir, homeDirErr = usr.HomeDir, nil
			return
		}
		homeDir, homeDirErr = home, nil
	})
	return homeDir, homeDirErr
}

func bailOnError(err error, format string, a ...interface{}) { // nolint: golint,goprintffuncname
	if err != nil {
		if format != "" {
			logrus.Errorf("%s: %v", fmt.Sprintf(format, a...), err)
		} else {
			logrus.Errorf("%v", err)
		}
		os.Exit(1)
	}
}
