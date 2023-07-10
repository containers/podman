//go:build !linux
// +build !linux

package criu

import "fmt"

func CheckForCriu(version int) error {
	return fmt.Errorf("CheckForCriu not supported on this platform")
}

func MemTrack() bool {
	return false
}

func GetCriuVersion() (int, error) {
	return MinCriuVersion, nil
}
