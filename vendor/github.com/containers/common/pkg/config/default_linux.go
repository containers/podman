package config

import (
	"syscall"
)

// isCgroup2UnifiedMode returns whether we are running in cgroup2 mode.
func isCgroup2UnifiedMode() (isUnified bool, isUnifiedErr error) {
	_cgroup2SuperMagic := int64(0x63677270)
	cgroupRoot := "/sys/fs/cgroup"

	var st syscall.Statfs_t
	if err := syscall.Statfs(cgroupRoot, &st); err != nil {
		isUnified, isUnifiedErr = false, err
	} else {
		isUnified, isUnifiedErr = st.Type == _cgroup2SuperMagic, nil
	}
	return
}
