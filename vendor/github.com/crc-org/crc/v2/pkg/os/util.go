package os

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/crc-org/crc/v2/pkg/crc/logging"
)

// ReplaceOrAddEnv changes the value of an environment variable if it exists otherwise add the new variable
// It drops the existing value and appends the new value in-place
func ReplaceOrAddEnv(variables []string, varName string, value string) []string {
	var result []string

	found := false
	for _, e := range variables {
		pair := strings.Split(e, "=")
		if pair[0] != varName {
			result = append(result, e)
		} else {
			found = true
			result = append(result, fmt.Sprintf("%s=%s", varName, value))
		}
	}

	if !found {
		result = append(result, fmt.Sprintf("%s=%s", varName, value))
	}
	return result
}

func CopyFileContents(src string, dst string, permission os.FileMode) error {
	logging.Debugf("Copying '%s' to '%s'", src, dst)
	srcFile, err := os.Open(filepath.Clean(src))
	if err != nil {
		return fmt.Errorf("[%v] Cannot open src file '%s'", err, src)
	}
	defer srcFile.Close()

	destFile, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE, permission)
	if err != nil {
		return fmt.Errorf("[%v] Cannot create dst file '%s'", err, dst)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return fmt.Errorf("[%v] Cannot copy '%s' to '%s'", err, src, dst)
	}

	err = destFile.Sync()
	if err != nil {
		return fmt.Errorf("[%v] Cannot sync '%s' to '%s'", err, src, dst)
	}

	return destFile.Close()
}

func FileContentMatches(path string, expectedContent []byte) error {
	_, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("File not found: %s: %s", path, err.Error())
	}
	content, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("Error opening file: %s: %s", path, err.Error())
	}
	if !bytes.Equal(content, expectedContent) {
		return fmt.Errorf("File has unexpected content: %s", path)

	}
	return nil
}

func WriteFileIfContentChanged(path string, newContent []byte, perm os.FileMode) (bool, error) {
	err := FileContentMatches(path, newContent)
	if err == nil {
		return false, nil
	}

	/* Intentionally ignore errors, just try to write the file if we can't read it */
	err = os.WriteFile(path, newContent, perm)

	if err != nil {
		return false, err
	}
	return true, nil
}

// FileExists returns true if the file at path exists.
// It returns false if it does not exist, or if there was an error when checking for its existence.
// This means there can be false negatives if Lstat fails because of permission issues (file exists,
// but is not reachable by the current user)
func FileExists(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func RemoveFileIfExists(path string) error {
	if FileExists(path) {
		return os.Remove(path)
	}
	return nil
}

func RunningUsingSSH() bool {
	return os.Getenv("SSH_TTY") != ""
}

// RemoveFileGlob takes a glob pattern as string to remove the files and directories that matches
func RemoveFileGlob(glob string) error {
	matchedFiles, err := filepath.Glob(glob)
	if err != nil {
		return fmt.Errorf("Unable to find matches: %w", err)
	}
	for _, file := range matchedFiles {
		if err = os.RemoveAll(file); err != nil {
			return fmt.Errorf("Failed to delete file: %w", err)
		}
	}
	return nil
}
