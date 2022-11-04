//go:build linux && !cgo
// +build linux,!cgo

package unshare

import (
	"os"
)

func getenv(name string) string {
	return os.Getenv(name)
}
