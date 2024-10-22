// Deprecated: use github.com/moby/sys/userns
package userns

import "github.com/moby/sys/userns"

// RunningInUserNS detects whether we are currently running in a Linux
// user namespace and memoizes the result. It returns false on non-Linux
// platforms.
//
// Deprecated: use [userns.RunningInUserNS].
func RunningInUserNS() bool {
	return userns.RunningInUserNS()
}
