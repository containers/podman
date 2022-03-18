//go:build !linux
// +build !linux

package retry

func isErrnoERESTART(e error) bool {
	return false
}
