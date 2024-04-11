//go:build !remote
// +build !remote

package registry

func init() {
	abiSupport = true
}
