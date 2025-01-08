//go:build !linux

package reflink

import (
	"io"
	"os"
)

// LinkOrCopy attempts to reflink the source to the destination fd.
// If reflinking fails or is unsupported, it falls back to io.Copy().
func LinkOrCopy(src, dst *os.File) error {
	_, err := io.Copy(dst, src)
	return err
}
