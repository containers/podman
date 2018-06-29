// +build !linux

package libpod

func moveConmonToCgroup() error {
	return ErrOSNotSupported
}
