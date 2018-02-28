// +build linux solaris

package storage

import (
	"time"

	"golang.org/x/sys/unix"
)

// TouchedSince indicates if the lock file has been touched since the specified time
func (l *lockfile) TouchedSince(when time.Time) bool {
	st := unix.Stat_t{}
	err := unix.Fstat(int(l.fd), &st)
	if err != nil {
		return true
	}
	touched := time.Unix(st.Mtim.Unix())
	return when.Before(touched)
}
