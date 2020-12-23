// +build linux darwin

package util

import (
	"os"
	"sync"
	"syscall"
)

type hardlinkDeviceAndInode struct {
	device, inode uint64
}

type HardlinkChecker struct {
	hardlinks sync.Map
}

func (h *HardlinkChecker) Check(fi os.FileInfo) string {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok && fi.Mode().IsRegular() && st.Nlink > 1 {
		if name, ok := h.hardlinks.Load(makeHardlinkDeviceAndInode(st)); ok && name.(string) != "" {
			return name.(string)
		}
	}
	return ""
}
func (h *HardlinkChecker) Add(fi os.FileInfo, name string) {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok && fi.Mode().IsRegular() && st.Nlink > 1 {
		h.hardlinks.Store(makeHardlinkDeviceAndInode(st), name)
	}
}

func UID(st os.FileInfo) int {
	return int(st.Sys().(*syscall.Stat_t).Uid)
}

func GID(st os.FileInfo) int {
	return int(st.Sys().(*syscall.Stat_t).Gid)
}
