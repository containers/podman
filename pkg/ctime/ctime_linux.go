// +build linux

package ctime

import (
	"os"
	"syscall"
	"time"
)

func created(fi os.FileInfo) time.Time {
	st := fi.Sys().(*syscall.Stat_t)
	return time.Unix(st.Ctim.Sec, st.Ctim.Nsec)
}
