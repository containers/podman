// +build linux

package util

import (
	"syscall"
)

func makeHardlinkDeviceAndInode(st *syscall.Stat_t) hardlinkDeviceAndInode {
	return hardlinkDeviceAndInode{
		device: st.Dev,
		inode:  st.Ino,
	}
}
