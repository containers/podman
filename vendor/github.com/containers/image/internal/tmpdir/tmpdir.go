package tmpdir

import (
	"os"
	"runtime"
)

// TemporaryDirectoryForBigFiles returns a directory for temporary (big) files.
// On non Windows systems it avoids the use of os.TempDir(), because the default temporary directory usually falls under /tmp
// which on systemd based systems could be the unsuitable tmpfs filesystem.
func TemporaryDirectoryForBigFiles() string {
	var temporaryDirectoryForBigFiles string
	if runtime.GOOS == "windows" {
		temporaryDirectoryForBigFiles = os.TempDir()
	} else {
		temporaryDirectoryForBigFiles = "/var/tmp"
	}
	return temporaryDirectoryForBigFiles
}
