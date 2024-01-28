//go:build (windows && ignore) || osx

package sysinfo

// NUMANodeCount queries the system for the count of Memory Nodes available
// for use to this process. Returns 0 on non NUMAs systems.
func NUMANodeCount() int {
	return 0
}
