//go:build !linux
// +build !linux

package buildah

func runningInUserNS() bool {
	return false
}
