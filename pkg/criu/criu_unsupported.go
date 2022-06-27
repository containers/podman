//go:build !linux
// +build !linux

package criu

func CheckForCriu(version int) bool {
	return false
}

func MemTrack() bool {
	return false
}

func GetCriuVersion() (int, error) {
	return MinCriuVersion, nil
}
