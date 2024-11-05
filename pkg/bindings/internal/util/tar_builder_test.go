package util

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/containers/storage/pkg/archive"
	"github.com/stretchr/testify/assert"
)

func TestTarBuilder(t *testing.T) {
	testCases := []struct {
		description string
		setup       func(tempDir string) []struct{ source, target string }
		excludes    []string
		validate    func(t *testing.T, destDir string)
		expectError bool
	}{
		{
			description: "single file",
			setup: func(tempDir string) []struct{ source, target string } {
				srcFile := filepath.Join(tempDir, "file1.txt")
				err := os.WriteFile(srcFile, []byte("hello"), 0644)
				assert.NoError(t, err)
				return []struct{ source, target string }{
					{source: srcFile, target: "file1.txt"},
				}
			},
			validate: func(t *testing.T, destDir string) {
				fileContent, err := os.ReadFile(filepath.Join(destDir, "file1.txt"))
				assert.NoError(t, err)
				assert.Equal(t, "hello", string(fileContent))
			},
		},
		{
			description: "multiple files with custom targets",
			setup: func(tempDir string) []struct{ source, target string } {
				srcFile1 := filepath.Join(tempDir, "file1.txt")
				srcFile2 := filepath.Join(tempDir, "file2.txt")
				err := os.WriteFile(srcFile1, []byte("hello1"), 0644)
				assert.NoError(t, err)
				err = os.WriteFile(srcFile2, []byte("hello2"), 0644)
				assert.NoError(t, err)
				return []struct{ source, target string }{
					{source: srcFile1, target: "dir1/file1.txt"},
					{source: srcFile2, target: "dir2/file2.txt"},
				}
			},
			validate: func(t *testing.T, destDir string) {
				file1Content, err := os.ReadFile(filepath.Join(destDir, "dir1/file1.txt"))
				assert.NoError(t, err)
				assert.Equal(t, "hello1", string(file1Content))

				file2Content, err := os.ReadFile(filepath.Join(destDir, "dir2/file2.txt"))
				assert.NoError(t, err)
				assert.Equal(t, "hello2", string(file2Content))
			},
		},
		{
			description: "nested directories",
			setup: func(tempDir string) []struct{ source, target string } {
				dir := filepath.Join(tempDir, "nested")
				err := os.Mkdir(dir, 0755)
				assert.NoError(t, err)
				err = os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("nested file1"), 0644)
				assert.NoError(t, err)
				err = os.Mkdir(filepath.Join(dir, "subdir"), 0755)
				assert.NoError(t, err)
				err = os.WriteFile(filepath.Join(dir, "subdir", "file2.txt"), []byte("nested file2"), 0644)
				assert.NoError(t, err)
				return []struct{ source, target string }{
					{source: dir, target: "nested"},
				}
			},
			validate: func(t *testing.T, destDir string) {
				file1Content, err := os.ReadFile(filepath.Join(destDir, "nested/file1.txt"))
				assert.NoError(t, err)
				assert.Equal(t, "nested file1", string(file1Content))

				file2Content, err := os.ReadFile(filepath.Join(destDir, "nested/subdir/file2.txt"))
				assert.NoError(t, err)
				assert.Equal(t, "nested file2", string(file2Content))
			},
		},
		{
			description: "empty directories",
			setup: func(tempDir string) []struct{ source, target string } {
				dir := filepath.Join(tempDir, "emptydir")
				err := os.Mkdir(dir, 0755)
				assert.NoError(t, err)
				return []struct{ source, target string }{
					{source: dir, target: "emptydir"},
				}
			},
			validate: func(t *testing.T, destDir string) {
				info, err := os.Stat(filepath.Join(destDir, "emptydir"))
				assert.NoError(t, err)
				assert.True(t, info.IsDir())
			},
		},
		{
			description: "exclude specific files",
			setup: func(tempDir string) []struct{ source, target string } {
				err := os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("file1"), 0644)
				assert.NoError(t, err)
				err = os.WriteFile(filepath.Join(tempDir, "file2.log"), []byte("file2"), 0644)
				assert.NoError(t, err)
				err = os.WriteFile(filepath.Join(tempDir, "file3.tmp"), []byte("file3"), 0644)
				assert.NoError(t, err)
				return []struct{ source, target string }{
					{source: tempDir, target: ""},
				}
			},
			excludes: []string{"*.log", "*.tmp"},
			validate: func(t *testing.T, destDir string) {
				_, err := os.Stat(filepath.Join(destDir, "file2.log"))
				assert.Error(t, err) // file2.log should be excluded

				_, err = os.Stat(filepath.Join(destDir, "file3.tmp"))
				assert.Error(t, err) // file3.tmp should be excluded

				file1Content, err := os.ReadFile(filepath.Join(destDir, "file1.txt"))
				assert.NoError(t, err)
				assert.Equal(t, "file1", string(file1Content))
			},
		},
		{
			description: "symlink handling",
			setup: func(tempDir string) []struct{ source, target string } {
				filePath := filepath.Join(tempDir, "file.txt")
				err := os.WriteFile(filePath, []byte("real file"), 0644)
				assert.NoError(t, err)
				linkPath := filepath.Join(tempDir, "symlink")
				err = os.Symlink("file.txt", linkPath)
				assert.NoError(t, err)
				return []struct{ source, target string }{
					{source: tempDir, target: ""},
				}
			},
			validate: func(t *testing.T, destDir string) {
				// Check the symlink exists
				linkDest, err := os.Readlink(filepath.Join(destDir, "symlink"))
				assert.NoError(t, err)
				assert.Equal(t, "file.txt", linkDest)

				// Check the file is also correctly copied
				fileContent, err := os.ReadFile(filepath.Join(destDir, "file.txt"))
				assert.NoError(t, err)
				assert.Equal(t, "real file", string(fileContent))
			},
		},
		{
			description: "multiple sources",
			setup: func(tempDir string) []struct{ source, target string } {
				srcFile1 := filepath.Join(tempDir, "source1", "file1.txt")
				srcFile2 := filepath.Join(tempDir, "source2", "file2.txt")
				err := os.Mkdir(filepath.Dir(srcFile1), 0755)
				assert.NoError(t, err)
				err = os.Mkdir(filepath.Dir(srcFile2), 0755)
				assert.NoError(t, err)
				err = os.WriteFile(srcFile1, []byte("hello1"), 0644)
				assert.NoError(t, err)
				err = os.WriteFile(srcFile2, []byte("hello2"), 0644)
				assert.NoError(t, err)
				return []struct{ source, target string }{
					{source: filepath.Dir(srcFile1), target: "target1"},
					{source: filepath.Dir(srcFile2), target: "target2"},
				}
			},
			validate: func(t *testing.T, destDir string) {
				file1Content, err := os.ReadFile(filepath.Join(destDir, "target1/file1.txt"))
				assert.NoError(t, err)
				assert.Equal(t, "hello1", string(file1Content))

				file2Content, err := os.ReadFile(filepath.Join(destDir, "target2/file2.txt"))
				assert.NoError(t, err)
				assert.Equal(t, "hello2", string(file2Content))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			// Create a temporary directory for the test setup
			tempDir := t.TempDir()

			// Set up the test files and directories
			sourceMappings := testCase.setup(tempDir)

			// Create a new TarBuilder
			tb := NewTarBuilder()

			// Add source mappings
			for _, mapping := range sourceMappings {
				err := tb.Add(mapping.source, mapping.target)
				assert.NoError(t, err)
			}

			// Add excludes, if any
			tb.Exclude(testCase.excludes...)

			// Build the tarball
			tarStream, err := tb.Build()
			if testCase.expectError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			defer tarStream.Close()

			// Create a temporary directory to untar the result
			destDir := t.TempDir()

			// Untar the resulting tarball
			err = untar(tarStream, destDir)
			assert.NoError(t, err)

			// Validate the output
			testCase.validate(t, destDir)
		})
	}
}

// Helper function to untar the resulting tarball
func untar(tarStream io.Reader, destDir string) error {
	return archive.Untar(tarStream, destDir, nil)
}
