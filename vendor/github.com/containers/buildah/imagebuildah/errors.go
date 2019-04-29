package imagebuildah

import "errors"

var (
	errDanglingSymlink = errors.New("error evaluating dangling symlink")
)
