//go:build (linux && !mips && !mipsle && !mips64 && !mips64le) || freebsd
// +build linux,!mips,!mipsle,!mips64,!mips64le freebsd

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
