//go:build windows

package machine

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CreateNewItemWithPowerShell creates a new item using PowerShell.
// It's an helper to easily create junctions on Windows (as well as other file types).
// It constructs a PowerShell command to create a new item at the specified path with the given item type.
// If a target is provided, it includes it in the command.
//
// Parameters:
//   - t: The testing.T instance.
//   - path: The path where the new item will be created.
//   - itemType: The type of the item to be created (e.g., "File", "SymbolicLink", "Junction").
//   - target: The target for the new item, if applicable.
func CreateNewItemWithPowerShell(t *testing.T, path string, itemType string, target string) {
	var pwshCmd string
	pwshCmd = "New-Item -Path " + path + " -ItemType " + itemType
	if target != "" {
		pwshCmd += " -Target " + target
	}
	cmd := exec.Command("pwsh", "-Command", pwshCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	require.NoError(t, err)
}

// TestEvalSymlinksOrClean tests the EvalSymlinksOrClean function.
// In particular it verifies that EvalSymlinksOrClean behaves as
// filepath.EvalSymlink before Go 1.23 - with the exception of
// files under a mount point (juntion) that aren't resolved
// anymore.
// The old behavior of filepath.EvalSymlinks can be tested with
// the directive "//go:debug winsymlink=0" and replacing EvalSymlinksOrClean()
// with filepath.EvalSymlink().
func TestEvalSymlinksOrClean(t *testing.T) {
	// Create a temporary directory to store the normal file
	normalFileDir := t.TempDir()

	// Create a temporary directory to store the (hard/sym)link files
	linkFilesDir := t.TempDir()

	// Create a temporary directory where the mount point will be created
	mountPointDir := t.TempDir()

	// Create a normal file
	normalFile := filepath.Join(normalFileDir, "testFile")
	CreateNewItemWithPowerShell(t, normalFile, "File", "")

	// Create a symlink file
	symlinkFile := filepath.Join(linkFilesDir, "testSymbolicLink")
	CreateNewItemWithPowerShell(t, symlinkFile, "SymbolicLink", normalFile)

	// Create a hardlink file
	hardlinkFile := filepath.Join(linkFilesDir, "testHardLink")
	CreateNewItemWithPowerShell(t, hardlinkFile, "HardLink", normalFile)

	// Create a mount point file
	mountPoint := filepath.Join(mountPointDir, "testJunction")
	mountPointFile := filepath.Join(mountPoint, "testFile")
	CreateNewItemWithPowerShell(t, mountPoint, "Junction", normalFileDir)

	// Replaces the backslashes with forward slashes in the normal file path
	normalFileWithBadSeparators := filepath.ToSlash(normalFile)

	tests := []struct {
		name     string
		filePath string
		want     string
	}{
		{
			name:     "Normal file",
			filePath: normalFile,
			want:     normalFile,
		},
		{
			name:     "File under a mount point (juntion)",
			filePath: mountPointFile,
			want:     mountPointFile,
		},
		{
			name:     "Symbolic link",
			filePath: symlinkFile,
			want:     normalFile,
		},
		{
			name:     "Hard link",
			filePath: hardlinkFile,
			want:     hardlinkFile,
		},
		{
			name:     "Bad separators in path",
			filePath: normalFileWithBadSeparators,
			want:     normalFile,
		},
	}

	for _, tt := range tests {
		assert := assert.New(t)
		t.Run(tt.name, func(t *testing.T) {
			got, err := EvalSymlinksOrClean(tt.filePath)
			require.NoError(t, err)
			assert.Equal(tt.want, got)
		})
	}
}
