// +build !arm,!386

package libpod

import (
	"os"
	"syscall"
	"time"
)

// Get the created time of a file
// Only works on 64-bit OSes
func getFinishedTime(fi os.FileInfo) time.Time {
	st := fi.Sys().(*syscall.Stat_t)
	return time.Unix(st.Ctim.Sec, st.Ctim.Nsec)
}
