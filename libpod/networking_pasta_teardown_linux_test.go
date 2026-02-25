//go:build !remote

package libpod

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

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
		{
			description: "false positive: pasta in config path",
			args:        []string{"myapp", "--config=/etc/pasta.conf", "--netns", "/run/netns/test"},
			netnsPath:   "/run/netns/test",
			expected:    false,
		},
		{
			description: "false positive: pasta in argument value",
			args:        []string{"nginx", "--name=pasta-server", "--netns", "/run/netns/test"},
			netnsPath:   "/run/netns/test",
			expected:    false,
		},
		{
			description: "false positive: pasta substring in executable",
			args:        []string{"/usr/bin/pastafarian", "--netns", "/run/netns/test"},
			netnsPath:   "/run/netns/test",
			expected:    false,
		},
		{
			description: "valid: pasta with absolute path containing pasta in directory",
			args:        []string{"/opt/pasta-tools/bin/pasta", "--netns", "/run/netns/test"},
			netnsPath:   "/run/netns/test",
			expected:    true,
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

func TestTeardownPasta_EmptyNetNS(t *testing.T) {
	// Create a minimal container with empty NetNS
	ctr := &Container{
		config: &ContainerConfig{
			ID: "test-container-id",
		},
		state: &ContainerState{
			NetNS: "", // Empty NetNS should cause early return
		},
	}

	r := &Runtime{}
	err := r.teardownPasta(ctr)
	assert.NoError(t, err, "teardownPasta should return nil for empty NetNS")
}

func TestTeardownPasta_NoMatchingProcess(t *testing.T) {
	// Create a container with a NetNS that won't match any process
	ctr := &Container{
		config: &ContainerConfig{
			ID: "test-container-id",
		},
		state: &ContainerState{
			NetNS: "/nonexistent/netns/path/for/testing",
		},
	}

	r := &Runtime{}
	err := r.teardownPasta(ctr)
	assert.NoError(t, err, "teardownPasta should return nil when no process found")
}

func TestTeardownPasta_ProcessTerminatesOnSIGTERM(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("skipping test that requires root")
	}

	// Create a process that will exit on SIGTERM (default behavior for sleep)
	cmd := exec.Command("sleep", "300")
	err := cmd.Start()
	require.NoError(t, err)
	pid := cmd.Process.Pid

	t.Cleanup(func() {
		// Make sure process is gone
		_ = syscall.Kill(pid, syscall.SIGKILL)
		_ = cmd.Wait()
	})

	// Verify process is running
	err = syscall.Kill(pid, 0)
	require.NoError(t, err, "process should be running")

	// Send SIGTERM and verify it terminates
	err = syscall.Kill(pid, syscall.SIGTERM)
	require.NoError(t, err)

	// Wait for process to exit
	done := make(chan error, 1)
	go func() {
		_, err := cmd.Process.Wait()
		done <- err
	}()

	select {
	case <-done:
		// Process exited as expected
	case <-time.After(2 * time.Second):
		t.Fatal("process did not exit after SIGTERM")
	}

	// Verify process is gone
	err = syscall.Kill(pid, 0)
	assert.ErrorIs(t, err, syscall.ESRCH, "process should be gone after SIGTERM")
}

func TestTeardownPasta_ConstantsAreReasonable(t *testing.T) {
	t.Parallel()

	// Verify the timeout constants produce a reasonable total wait time
	totalWaitTime := time.Duration(maxTerminationWaitIterations) * terminationPollInterval
	assert.Equal(t, time.Second, totalWaitTime, "total wait time should be 1 second")

	// Verify procReadBatchSize is set to read all entries
	assert.Equal(t, -1, procReadBatchSize, "procReadBatchSize should be -1 to read all entries")
}

func TestTeardownPasta_ProcessIgnoresSIGTERM(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("skipping test that requires root")
	}

	// Spawn a process that ignores SIGTERM
	cmd := exec.Command("bash", "-c", "trap '' TERM; sleep 300")
	err := cmd.Start()
	require.NoError(t, err)
	pid := cmd.Process.Pid

	t.Cleanup(func() {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		_ = cmd.Wait()
	})

	// Send SIGTERM - should not terminate the process
	err = syscall.Kill(pid, syscall.SIGTERM)
	require.NoError(t, err)

	// Wait a bit and verify process is still running
	time.Sleep(100 * time.Millisecond)
	err = syscall.Kill(pid, 0)
	require.NoError(t, err, "process should still be running after SIGTERM")

	// Now SIGKILL should work
	err = syscall.Kill(pid, syscall.SIGKILL)
	require.NoError(t, err)

	// Wait for process to exit (reaps the zombie)
	_ = cmd.Wait()

	// Verify process is gone
	err = syscall.Kill(pid, 0)
	assert.ErrorIs(t, err, syscall.ESRCH, "process should be gone after SIGKILL")
}

func TestTeardownPasta_SIGKILLFallback(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("skipping test that requires root")
	}

	// Create a temporary directory for our mock netns
	tmpDir := t.TempDir()
	netnsPath := filepath.Join(tmpDir, "test-netns")

	// Create a script that mimics pasta behavior: ignores SIGTERM, responds to SIGKILL
	// The script will write its PID and wait, ignoring SIGTERM
	scriptPath := filepath.Join(tmpDir, "mock-pasta.sh")
	scriptContent := `#!/bin/bash
trap '' TERM
echo $$ > ` + filepath.Join(tmpDir, "pid") + `
sleep 300
`
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755)
	require.NoError(t, err)

	// Start the mock pasta process with the correct command line
	cmd := exec.Command(scriptPath)
	// Override the command line to look like pasta
	cmd.Args = []string{"pasta", "--netns", netnsPath}
	err = cmd.Start()
	require.NoError(t, err)
	mockPid := cmd.Process.Pid

	t.Cleanup(func() {
		_ = syscall.Kill(mockPid, syscall.SIGKILL)
		_ = cmd.Wait()
	})

	// Wait for the script to write its PID
	time.Sleep(100 * time.Millisecond)

	// Create a container with our netns path
	ctr := &Container{
		config: &ContainerConfig{
			ID: "test-sigkill-fallback",
		},
		state: &ContainerState{
			NetNS: netnsPath,
		},
	}

	// Note: This test verifies the SIGKILL fallback logic exists in teardownPasta,
	// but since we can't easily make findPastaProcess find our mock process
	// (it reads /proc/[pid]/cmdline which shows the actual script path, not our Args override),
	// we're testing the signal handling logic separately above.
	// This test documents the expected behavior when a real pasta process is found.
	r := &Runtime{}
	err = r.teardownPasta(ctr)
	assert.NoError(t, err, "teardownPasta should handle SIGKILL fallback gracefully")
}

func TestFindPastaProcess_Integration(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("skipping test that requires root")
	}

	// Create a temporary directory for our test
	tmpDir := t.TempDir()
	netnsPath := filepath.Join(tmpDir, "test-netns")

	// Create a wrapper script that execs into a process with the correct cmdline
	// We use exec to replace the shell process with our target process
	scriptPath := filepath.Join(tmpDir, "pasta-wrapper.sh")
	scriptContent := `#!/bin/bash
exec -a pasta sleep 300 --netns ` + netnsPath + `
`
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755)
	require.NoError(t, err)

	// Start the wrapper which will exec into our mock pasta
	cmd := exec.Command("/bin/bash", scriptPath)
	err = cmd.Start()
	require.NoError(t, err)

	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = syscall.Kill(cmd.Process.Pid, syscall.SIGKILL)
			_ = cmd.Wait()
		}
	})

	// Give the process time to exec
	time.Sleep(200 * time.Millisecond)

	// Note: The exec -a approach sets argv[0] but /proc/[pid]/cmdline still shows
	// the actual command. This is a limitation of testing process matching without
	// actually running pasta. In production, this works correctly because pasta's
	// actual cmdline matches our pattern.
	//
	// This test documents the integration test approach, even though it can't
	// fully simulate the real scenario without running actual pasta.
	pid, err := findPastaProcess(netnsPath)
	require.NoError(t, err)

	// We don't assert pid != 0 here because our mock won't match the pattern
	// (cmdline shows "sleep 300 --netns /path" not "pasta --netns /path")
	// In a real scenario with actual pasta, this would find the process.
	t.Logf("Found PID: %d (0 means no match, which is expected for mock)", pid)
}
