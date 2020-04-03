package unshare

import (
	"fmt"
	"os"
	"os/user"

	"github.com/pkg/errors"
)

// HomeDir returns the home directory for the current user.
func HomeDir() (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		usr, err := user.LookupId(fmt.Sprintf("%d", GetRootlessUID()))
		if err != nil {
			return "", errors.Wrapf(err, "unable to resolve HOME directory")
		}
		home = usr.HomeDir
	}
	return home, nil
}
