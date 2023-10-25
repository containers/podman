//go:build !linux && !remote
// +build !linux,!remote

package emulation

func registeredBinfmtMisc() ([]string, error) {
	return nil, nil
}
