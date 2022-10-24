//go:build go1.16
// +build go1.16

package toml

import (
	"io/fs"
)

// DecodeFS reads the contents of a file from [fs.FS] and decodes it with
// [Decode].
func DecodeFS(fsys fs.FS, path string, v interface{}) (MetaData, error) {
	fp, err := fsys.Open(path)
	if err != nil {
		return MetaData{}, err
	}
	defer fp.Close()
	return NewDecoder(fp).Decode(v)
}
