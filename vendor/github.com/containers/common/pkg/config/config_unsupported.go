//go:build !linux
// +build !linux

package config

func selinuxEnabled() bool {
	return false
}
