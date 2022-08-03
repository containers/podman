// +build linux,!mips,!mipsle,!mips64,!mips64le

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
