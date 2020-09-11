// +build !windows

package rusage

import (
	"syscall"
	"time"

	"github.com/pkg/errors"
)

func mkduration(tv syscall.Timeval) time.Duration {
	return time.Duration(tv.Sec)*time.Second + time.Duration(tv.Usec)*time.Microsecond
}

func get() (Rusage, error) {
	var rusage syscall.Rusage
	err := syscall.Getrusage(syscall.RUSAGE_CHILDREN, &rusage)
	if err != nil {
		return Rusage{}, errors.Wrapf(err, "error getting resource usage")
	}
	r := Rusage{
		Date:     time.Now(),
		Utime:    mkduration(rusage.Utime),
		Stime:    mkduration(rusage.Stime),
		Inblock:  int64(rusage.Inblock), // nolint: unconvert
		Outblock: int64(rusage.Oublock), // nolint: unconvert
	}
	return r, nil
}

// Supported returns true if resource usage counters are supported on this OS.
func Supported() bool {
	return true
}
