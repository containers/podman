//go:build !linux
// +build !linux

package emulation

func registeredBinfmtMisc() ([]string, error) {
	return nil, nil
}
