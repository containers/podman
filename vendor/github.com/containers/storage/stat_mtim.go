// +build linux solaris

package storage

import (
	"golang.org/x/sys/unix"
)

func statTMtimeUnix(st unix.Stat_t) (int64, int64) {
	return st.Mtim.Unix()
}
