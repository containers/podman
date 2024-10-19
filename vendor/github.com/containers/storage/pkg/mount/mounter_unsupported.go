//go:build !linux && !(freebsd && cgo)
// +build !linux
// +build !freebsd !cgo

package mount

func mount(device, target, mType string, flag uintptr, data string) error {
	panic("Not implemented")
}
