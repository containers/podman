package images

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTempFileManager(t *testing.T) {
	manager := NewTempFileManager()

	t.Run("CreateTempFileFromStdin", func(t *testing.T) {
		oldStdin := os.Stdin
		defer func() { os.Stdin = oldStdin }()

		r, w, err := os.Pipe()
		assert.NoError(t, err)
		os.Stdin = r

		_, err = w.WriteString("test content")
		assert.NoError(t, err)
		w.Close()

		filename, err := manager.CreateTempFileFromStdin("")
		assert.NoError(t, err)
		assert.NotEmpty(t, filename)

		manager.Cleanup()
	})

	t.Run("CreateTempSecret", func(t *testing.T) {
		secretPath := "testdata/secret.txt"
		contextDir := "testdata"

		filename, err := manager.CreateTempSecret(secretPath, contextDir)
		assert.NoError(t, err)
		assert.NotEmpty(t, filename)

		manager.Cleanup()
	})
}
