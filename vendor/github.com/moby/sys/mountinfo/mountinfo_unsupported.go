// +build !windows,!linux,!freebsd freebsd,!cgo

package mountinfo

import (
	"fmt"
	"io"
	"runtime"
)

var errNotImplemented = fmt.Errorf("not implemented on %s/%s", runtime.GOOS, runtime.GOARCH)

func parseMountTable(_ FilterFunc) ([]*Info, error) {
	return nil, errNotImplemented
}

func parseInfoFile(_ io.Reader, f FilterFunc) ([]*Info, error) {
	return parseMountTable(f)
}

func mounted(path string) (bool, error) {
	return false, errNotImplemented
}
