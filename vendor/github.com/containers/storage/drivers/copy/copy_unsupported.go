// +build !linux !cgo

package copy

import "github.com/containers/storage/pkg/chrootarchive"

// Mode indicates whether to use hardlink or copy content
type Mode int

const (
	// Content creates a new file, and copies the content of the file
	Content Mode = iota
)

// DirCopy copies or hardlinks the contents of one directory to another,
// properly handling soft links
func DirCopy(srcDir, dstDir string, _ Mode, _ bool) error {
	return chrootarchive.NewArchiver(nil).CopyWithTar(srcDir, dstDir)
}
