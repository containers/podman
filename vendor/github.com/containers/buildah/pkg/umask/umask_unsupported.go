// +build !linux,!darwin

package umask

func CheckUmask() {}

func SetUmask(int) int { return 0 }
