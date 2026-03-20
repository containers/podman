//go:build !remote

package libpod

// teardownPasta is a no-op on FreeBSD since pasta is Linux-only.
func (r *Runtime) teardownPasta(_ *Container) error {
	return nil
}
