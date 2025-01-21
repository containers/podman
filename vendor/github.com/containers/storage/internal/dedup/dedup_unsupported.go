//go:build !linux

package dedup

import (
	"io/fs"
)

type dedupFiles struct{}

func newDedupFiles() (*dedupFiles, error) {
	return nil, notSupported
}

// isFirstVisitOf records that the file is being processed.  Returns true if the file was already visited.
func (d *dedupFiles) isFirstVisitOf(fi fs.FileInfo) (bool, error) {
	return false, notSupported
}

// dedup deduplicates the file at src path to dst path
func (d *dedupFiles) dedup(src, dst string, fiDst fs.FileInfo) (uint64, error) {
	return 0, notSupported
}

func readAllFile(path string, info fs.FileInfo, fn func([]byte) (string, error)) (string, error) {
	return "", notSupported
}
