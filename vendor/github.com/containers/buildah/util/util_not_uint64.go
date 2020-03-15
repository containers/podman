// +build darwin

package util

import (
	"syscall"
)

func makeHardlinkDeviceAndInode(st *syscall.Stat_t) hardlinkDeviceAndInode {
	return hardlinkDeviceAndInode{
		device: uint64(st.Dev),
		inode:  uint64(st.Ino),
	}
}
