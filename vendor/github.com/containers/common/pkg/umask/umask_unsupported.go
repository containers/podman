//go:build !linux && !darwin
// +build !linux,!darwin

package umask

func Check() {}

func Set(int) int { return 0 }
