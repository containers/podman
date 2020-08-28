package mount

import (
	"github.com/containers/storage/pkg/fileutils"
	"github.com/moby/sys/mountinfo"
)

type Info = mountinfo.Info

func GetMounts() ([]*Info, error) {
	return mountinfo.GetMounts(nil)
}

// Mounted determines if a specified mountpoint has been mounted.
func Mounted(mountpoint string) (bool, error) {
	mountpoint, err := fileutils.ReadSymlinkedPath(mountpoint)
	if err != nil {
		return false, err
	}
	return mountinfo.Mounted(mountpoint)
}
