// +build !linux

package libpod

import (
	"os"
	"time"
)

func getFinishedTime(fi os.FileInfo) time.Time {
	return time.Time{}
}
