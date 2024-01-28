//go:build linux

package sysinfo

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// NUMANodeCount queries the system for the count of Memory Nodes available
// for use to this process.
func NUMANodeCount() int {
	// this is the correct flag name (not defined in the unix package)
	//nolint:revive
	MPOL_F_MEMS_ALLOWED := (1 << 2)
	var mask [1024 / 64]uintptr
	_, _, err := unix.RawSyscall6(unix.SYS_GET_MEMPOLICY, 0, uintptr(unsafe.Pointer(&mask[0])), uintptr(len(mask)*8), 0, uintptr(MPOL_F_MEMS_ALLOWED), 0)
	if err != 0 {
		return 0
	}

	// For every available thread a bit is set in the mask.
	nmem := 0
	for _, e := range mask {
		if e == 0 {
			continue
		}
		nmem += int(popcnt(uint64(e)))
	}
	return nmem
}
