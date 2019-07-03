//+build linux

package createconfig

import (
	"syscall"

	"github.com/pkg/errors"
)

type systemRlimit struct {
	name  string
	value int
}

var systemLimits = []systemRlimit{
	{"RLIMIT_AS", syscall.RLIMIT_AS},
	{"RLIMIT_CORE", syscall.RLIMIT_CORE},
	{"RLIMIT_CPU", syscall.RLIMIT_CPU},
	{"RLIMIT_DATA", syscall.RLIMIT_DATA},
	{"RLIMIT_FSIZE", syscall.RLIMIT_FSIZE},
	{"RLIMIT_NOFILE", syscall.RLIMIT_NOFILE},
	{"RLIMIT_STACK", syscall.RLIMIT_STACK},
}

func getHostRlimits() ([]systemUlimit, error) {
	ret := []systemUlimit{}
	for _, i := range systemLimits {
		var l syscall.Rlimit
		if err := syscall.Getrlimit(i.value, &l); err != nil {
			return nil, errors.Wrapf(err, "cannot read limits for %s", i.name)
		}
		s := systemUlimit{
			name: i.name,
			max:  l.Max,
			cur:  l.Cur,
		}
		ret = append(ret, s)
	}
	return ret, nil

}
