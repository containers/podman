// +build !linux

package chrootuser

import (
	"github.com/pkg/errors"
)

func lookupUserInContainer(rootdir, username string) (uint64, uint64, error) {
	return 0, 0, errors.New("user lookup not supported")
}

func lookupGroupInContainer(rootdir, groupname string) (uint64, error) {
	return 0, errors.New("group lookup not supported")
}

func lookupGroupForUIDInContainer(rootdir string, userid uint64) (string, uint64, error) {
	return "", 0, errors.New("primary group lookup by uid not supported")
}
