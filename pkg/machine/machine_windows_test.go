//go:build windows

package machine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shortPathToLongPath converts a Windows short path (C:\PROGRA~1) to its
// long path equivalent (C:\Program Files). It returns an error if shortPath
// doesn't exist.
func shortPathToLongPath(shortPath string) (string, error) {
	shortPathPtr, err := windows.UTF16PtrFromString(shortPath)
	if err != nil {
		return "", err
	}
	len, err := windows.GetLongPathName(shortPathPtr, nil, 0)
	if err != nil {
		return "", err
	}
	if len == 0 {
		return "", fmt.Errorf("failed to get buffer size for path: %s", shortPath)
	}
	longPathPtr := &(make([]uint16, len)[0])
	_, err = windows.GetLongPathName(shortPathPtr, longPathPtr, len)
	if err != nil {
		return "", err
	}
	return windows.UTF16PtrToString(longPathPtr), nil
}

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
	var pwshCmd, pwshPath string
	// Look for Powershell 7 first as it allow Symlink creation for non-admins too
	pwshPath, err := exec.LookPath("pwsh.exe")
	if err != nil {
		// Use Powershell 5 that is always present
		pwshPath = "powershell.exe"
	}
	pwshCmd = "New-Item -Path " + path + " -ItemType " + itemType
	if target != "" {
		pwshCmd += " -Target " + target
	}
	cmd := exec.Command(pwshPath, "-Command", pwshCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
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
	normalFileDir, err := shortPathToLongPath(t.TempDir())
	require.NoError(t, err)

	// Create a temporary directory to store the (hard/sym)link files
	linkFilesDir, err := shortPathToLongPath(t.TempDir())
	require.NoError(t, err)

	// Create a temporary directory where the mount point will be created
	mountPointDir, err := shortPathToLongPath(t.TempDir())
	require.NoError(t, err)

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
