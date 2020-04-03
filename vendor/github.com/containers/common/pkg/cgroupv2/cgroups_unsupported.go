// +build !linux

package cgroupv2

// Enabled returns whether we are running in cgroup 2 cgroup2 mode.
func Enabled() (bool, error) {
	return false, nil
}
