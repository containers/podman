package unshare

import (
	"fmt"
	"os"
	"os/user"
	"sync"

	"github.com/pkg/errors"
)

var (
	homeDirOnce sync.Once
	homeDirErr  error
	homeDir     string
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
		}
		homeDir, homeDirErr = home, nil
	})
	return homeDir, homeDirErr
}
