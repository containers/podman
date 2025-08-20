package images

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

// TempFileManager manages temporary files created during image build.
// It maintains a list of created temporary files and provides cleanup functionality
// to ensure proper resource management.
type TempFileManager struct {
	files []string
}

func NewTempFileManager() *TempFileManager {
	return &TempFileManager{
		files: make([]string, 0),
	}
}

func (t *TempFileManager) AddFile(filename string) {
	t.files = append(t.files, filename)
}

func (t *TempFileManager) Cleanup() {
	for _, file := range t.files {
		if err := os.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
			logrus.Errorf("Failed to remove temp file %s: %v", file, err)
		}
	}
	t.files = t.files[:0] // Reset slice
}

// CreateTempFileFromStdin reads content from stdin and creates a temporary file
// in the specified destination directory. The temporary file is automatically
// added to the manager's cleanup list.
//
// Parameters:
//   - dest: The directory where the temporary file should be created
//
// Returns:
//   - string: The path to the created temporary file
//   - error: Any error encountered during the operation
func (t *TempFileManager) CreateTempFileFromStdin(dest string) (string, error) {
	content, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("reading from stdin: %w", err)
	}

	tmpFile, err := os.CreateTemp(dest, "build-stdin-*")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	defer tmpFile.Close()

	filename := tmpFile.Name()
	t.AddFile(filename)

	if _, err := tmpFile.Write(content); err != nil {
		return "", fmt.Errorf("writing to temp file: %w", err)
	}

	return filename, nil
}

// CreateTempSecret creates a temporary copy of a secret file in the specified
// context directory. The original secret file is copied to a new temporary file
// which is automatically added to the manager's cleanup list.
//
// Parameters:
//   - secretPath: The path to the source secret file to copy
//   - contextDir: The directory where the temporary secret file should be created
//
// Returns:
//   - string: The path to the created temporary secret file
//   - error: Any error encountered during the operation
func (t *TempFileManager) CreateTempSecret(secretPath, contextDir string) (string, error) {
	tmpSecretFile, err := os.CreateTemp(contextDir, "podman-build-secret-*")
	if err != nil {
		return "", fmt.Errorf("creating temp secret file: %w", err)
	}
	defer tmpSecretFile.Close()

	filename := tmpSecretFile.Name()
	t.AddFile(filename)

	srcSecretFile, err := os.Open(secretPath)
	if err != nil {
		tmpSecretFile.Close()
		return "", fmt.Errorf("opening secret file %s: %w", secretPath, err)
	}
	defer srcSecretFile.Close()

	if _, err := io.Copy(tmpSecretFile, srcSecretFile); err != nil {
		return "", fmt.Errorf("copying secret content: %w", err)
	}

	return filename, nil
}
