// +build cgo

package archive

import (
	"io"

	"github.com/DataDog/zstd"
)

func zstdReader(buf io.Reader) (io.ReadCloser, error) {
	return zstd.NewReader(buf), nil
}

func zstdWriter(dest io.Writer) (io.WriteCloser, error) {
	return zstd.NewWriter(dest), nil
}
