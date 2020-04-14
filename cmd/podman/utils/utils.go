package utils

import "os"

// IsDir returns true if the specified path refers to a directory.
func IsDir(path string) bool {
	file, err := os.Stat(path)
	if err != nil {
		return false
	}
	return file.IsDir()
}

// FileExists returns true if path refers to an existing file.
func FileExists(path string) bool {
	file, err := os.Stat(path)
	// All errors return file == nil
	if err != nil {
		return false
	}
	return !file.IsDir()
}
