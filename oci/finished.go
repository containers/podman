// +build !arm,!386

package oci

import (
	"os"
	"syscall"
	"time"
)

func getFinishedTime(fi os.FileInfo) time.Time {
	st := fi.Sys().(*syscall.Stat_t)
	return time.Unix(st.Ctim.Sec, st.Ctim.Nsec)
}
