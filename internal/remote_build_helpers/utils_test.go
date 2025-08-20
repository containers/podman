package remote_build_helpers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTempFileManager(t *testing.T) {
	manager := NewTempFileManager()

	t.Run("CreateTempFileFromReader", func(t *testing.T) {
		content := "test content"
		r := strings.NewReader(content)

		filename, err := manager.CreateTempFileFromReader("", "podman-build-stdin-*", r)
		assert.NoError(t, err)
		assert.FileExists(t, filename)

		data, err := os.ReadFile(filename)
		assert.NoError(t, err)
		assert.Equal(t, content, string(data))

		manager.Cleanup()

		assert.NoFileExists(t, filename)
	})

	t.Run("CreateTempSecret", func(t *testing.T) {
		tempdir := t.TempDir()
		secretPath := filepath.Join(tempdir, "secret.txt")

		content := "test secret"
		err := os.WriteFile(secretPath, []byte(content), 0600)
		assert.NoError(t, err)

		filename, err := manager.CreateTempSecret(secretPath, tempdir)
		assert.NoError(t, err)
		assert.FileExists(t, filename)

		data, err := os.ReadFile(filename)
		assert.NoError(t, err)
		assert.Equal(t, content, string(data))

		manager.Cleanup()

		assert.NoFileExists(t, filename)
	})
}
