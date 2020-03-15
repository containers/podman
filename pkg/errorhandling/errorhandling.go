package errorhandling

import (
	"os"

	"github.com/sirupsen/logrus"
)

// SyncQuiet syncs a file and logs any error. Should only be used within
// a defer.
func SyncQuiet(f *os.File) {
	if err := f.Sync(); err != nil {
		logrus.Errorf("unable to sync file %s: %q", f.Name(), err)
	}
}

// CloseQuiet closes a file and logs any error. Should only be used within
// a defer.
func CloseQuiet(f *os.File) {
	if err := f.Close(); err != nil {
		logrus.Errorf("unable to close file %s: %q", f.Name(), err)
	}
}
