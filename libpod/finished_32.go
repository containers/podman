// +build arm 386

package libpod

import (
	"os"
	"syscall"
	"time"
)

// Get created time of a file
// Only works on 32-bit OSes
func getFinishedTime(fi os.FileInfo) time.Time {
	st := fi.Sys().(*syscall.Stat_t)
	return time.Unix(int64(st.Ctim.Sec), int64(st.Ctim.Nsec))
}
