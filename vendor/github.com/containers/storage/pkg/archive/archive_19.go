//go:build !go1.10

package archive

import (
	"github.com/containers/storage/pkg/archive/hacktar"
)

func copyPassHeader(hdr *tar.Header) {
}

func maybeTruncateHeaderModTime(hdr *tar.Header) {
}
