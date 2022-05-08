//go:build !linux
// +build !linux

package criu

func CheckForCriu(version int) bool {
	return false
}

func MemTrack() bool {
	return false
}

func GetCriuVestion() (int, error) {
	return MinCriuVersion, nil
}
