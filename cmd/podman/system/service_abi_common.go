//go:build !linux && !remote

package system

// There is no cgroup on non linux.
func maybeMoveToSubCgroup() {}
