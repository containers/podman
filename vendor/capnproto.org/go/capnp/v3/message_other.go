//go:build !go1.8
// +build !go1.8

package capnp

func (e *Encoder) write(bufs [][]byte) error {
	for _, b := range bufs {
		if _, err := e.w.Write(b); err != nil {
			return err
		}
	}
	return nil
}
