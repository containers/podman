// +build !linux

package libpod

func (c *Container) cleanupCgroups() error {
	return ErrOSNotSupported
}
