// +build !cgo

package archive

import (
	"fmt"
	"io"
)

func zstdReader(buf io.Reader) (io.ReadCloser, error) {
	return nil, fmt.Errorf("zstd not supported on this platform")
}

func zstdWriter(dest io.Writer) (io.WriteCloser, error) {
	return nil, fmt.Errorf("zstd not supported on this platform")
}
