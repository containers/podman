//go:build !linux
// +build !linux

package criu

func MemTrack() bool {
	return false
}
