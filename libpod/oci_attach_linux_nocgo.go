//+build linux,!cgo

package libpod

func unixPathLength() int {
	return 107
}
