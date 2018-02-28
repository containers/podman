// +build darwin freebsd

package storage

import (
	"time"

	"golang.org/x/sys/unix"
)

func (l *lockfile) TouchedSince(when time.Time) bool {
	st := unix.Stat_t{}
	err := unix.Fstat(int(l.fd), &st)
	if err != nil {
		return true
	}
	touched := time.Unix(st.Mtimespec.Unix())
	return when.Before(touched)
}
