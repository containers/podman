//go:build !remote

package libpod

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchPastaCmdline(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description string
		args        []string
		netnsPath   string
		expected    bool
	}{
		{
			description: "valid pasta with matching netns",
			args:        []string{"pasta", "--netns", "/run/netns/test"},
			netnsPath:   "/run/netns/test",
			expected:    true,
		},
		{
			description: "valid pasta with wrong netns",
			args:        []string{"pasta", "--netns", "/run/netns/other"},
			netnsPath:   "/run/netns/test",
			expected:    false,
		},
		{
			description: "pasta without --netns flag",
			args:        []string{"pasta", "-p", "8080"},
			netnsPath:   "/run/netns/test",
			expected:    false,
		},
		{
			description: "non-pasta process with matching netns",
			args:        []string{"nginx", "--netns", "/run/netns/test"},
			netnsPath:   "/run/netns/test",
			expected:    false,
		},
		{
			description: "pasta in full path",
			args:        []string{"/usr/bin/pasta", "--netns", "/run/netns/test"},
			netnsPath:   "/run/netns/test",
			expected:    true,
		},
		{
			description: "empty args",
			args:        []string{},
			netnsPath:   "/run/netns/test",
			expected:    false,
		},
		{
			description: "--netns at end without value",
			args:        []string{"pasta", "--netns"},
			netnsPath:   "/run/netns/test",
			expected:    false,
		},
		{
			description: "empty netns path",
			args:        []string{"pasta", "--netns", "/run/netns/test"},
			netnsPath:   "",
			expected:    false,
		},
		{
			description: "pasta with multiple flags before netns",
			args:        []string{"pasta", "-t", "8080", "-u", "5353", "--netns", "/run/netns/ctr1"},
			netnsPath:   "/run/netns/ctr1",
			expected:    true,
		},
		{
			description: "pasta with flags after netns",
			args:        []string{"pasta", "--netns", "/run/netns/ctr1", "-t", "8080"},
			netnsPath:   "/run/netns/ctr1",
			expected:    true,
		},
		{
			description: "nil args treated as empty",
			args:        nil,
			netnsPath:   "/run/netns/test",
			expected:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			result := matchPastaCmdline(tc.args, tc.netnsPath)
			assert.Equal(t, tc.expected, result, "matchPastaCmdline(%v, %q)", tc.args, tc.netnsPath)
		})
	}
}

func TestFindPastaProcess_NoMatch(t *testing.T) {
	// Test that findPastaProcess returns 0 when no matching process exists
	// Use a netns path that won't match any real process
	pid, err := findPastaProcess("/nonexistent/netns/path/that/should/never/match")
	require.NoError(t, err)
	assert.Equal(t, 0, pid, "expected no matching process")
}

func TestFindPastaProcess_WithMockProcess(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("skipping test that requires root to reliably spawn processes")
	}

	// Create a unique netns path for this test
	netnsPath := filepath.Join(t.TempDir(), "test-netns")

	// Spawn a process that looks like pasta with our netns
	// We use 'sh -c' with exec to replace shell with a process that has
	// the desired argv[0]. However, /proc/PID/cmdline shows actual args.
	// Instead, we'll spawn a simple sleep and won't be able to match it
	// since we can't easily fake the cmdline.
	//
	// For a true integration test, we'd need to actually run pasta or
	// create a wrapper script. For now, we test the no-match case works.
	cmd := exec.Command("sleep", "60")
	err := cmd.Start()
	require.NoError(t, err)

	// Ensure cleanup
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})

	// This should not find our sleep process since it's not pasta
	pid, err := findPastaProcess(netnsPath)
	require.NoError(t, err)
	assert.Equal(t, 0, pid, "sleep process should not match pasta search")
}
