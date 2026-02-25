//go:build !remote

// SPDX-License-Identifier: Apache-2.0
//
// networking_pasta_teardown_linux.go - Teardown pasta(1) process
//
// Copyright (c) 2026 IBM Corporation

// Package libpod provides pasta process teardown functionality.
// This is necessary because pasta processes may not exit immediately when their
// network namespace is deleted, which can cause "address already in use" errors
// when containers are restarted. This module explicitly terminates pasta processes
// to ensure clean network state transitions.
package libpod

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// maxTerminationWaitIterations is the maximum number of iterations to wait for
	// pasta process termination after SIGTERM before sending SIGKILL
	maxTerminationWaitIterations = 50

	// terminationPollInterval is the interval between checks for process termination
	terminationPollInterval = 20 * time.Millisecond

	// procReadBatchSize is the number of /proc entries to read at once
	// -1 means read all entries at once, which is acceptable for /proc
	procReadBatchSize = -1
)

// teardownPasta terminates the pasta process for the given container.
// This is necessary because pasta may not exit immediately when the netns
// is deleted, causing "address already in use" errors on restart.
func (r *Runtime) teardownPasta(ctr *Container) error {
	if ctr.state.NetNS == "" {
		return nil
	}

	// Find the pasta process by looking for processes with our netns path
	pid, err := findPastaProcess(ctr.state.NetNS)
	if err != nil {
		logrus.Debugf("Could not find pasta process for container %s: %v", ctr.ID(), err)
		return nil // Not finding the process is not an error
	}

	if pid == 0 {
		logrus.Debugf("No pasta process found for container %s", ctr.ID())
		return nil
	}

	logrus.Debugf("Found pasta process %d for container %s, terminating", pid, ctr.ID())

	// Send SIGTERM to the pasta process
	// Note: There's a potential TOCTOU race between finding the process and signaling it,
	// but this is acceptable as the worst case is an ESRCH error which we handle gracefully.
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		if err == syscall.ESRCH {
			// Process already gone
			return nil
		}
		// Log the error but continue to wait loop - the process might still exit
		logrus.Warnf("Failed to send SIGTERM to pasta process %d: %v, will continue waiting", pid, err)
	}

	// Wait for the process to exit (with timeout)
	// Total wait time: maxTerminationWaitIterations * terminationPollInterval = 1 second
	for i := 0; i < maxTerminationWaitIterations; i++ {
		if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
			logrus.Debugf("Pasta process %d exited successfully", pid)
			return nil
		}
		time.Sleep(terminationPollInterval)
	}

	// If still running after timeout, send SIGKILL
	logrus.Warnf("Pasta process %d did not exit after SIGTERM, sending SIGKILL", pid)
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		return fmt.Errorf("failed to send SIGKILL to pasta process %d: %w", pid, err)
	}

	// Wait briefly for SIGKILL to take effect before returning.
	// This ensures the port is actually freed before we proceed.
	for i := 0; i < 10; i++ {
		if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
			logrus.Debugf("Pasta process %d terminated after SIGKILL", pid)
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Process still exists after 100ms - unusual but not fatal, log and continue
	logrus.Warnf("Pasta process %d may still be running after SIGKILL", pid)
	return nil
}

// matchPastaCmdline checks if the given command line arguments represent a pasta
// process using the specified network namespace path.
// Returns true if the args contain "pasta" as the executable name (exact match or
// path ending with "/pasta") AND have "--netns" followed by the matching netnsPath.
// This prevents false positives from processes that merely have "pasta" in their
// arguments (e.g., "--config=/etc/pasta.conf").
func matchPastaCmdline(args []string, netnsPath string) bool {
	if len(args) == 0 || netnsPath == "" {
		return false
	}

	isPasta := false
	hasOurNetns := false

	for i, arg := range args {
		// Match only if arg is exactly "pasta" or ends with "/pasta" (full path)
		if arg == "pasta" || strings.HasSuffix(arg, "/pasta") {
			isPasta = true
		}
		if arg == "--netns" && i+1 < len(args) && args[i+1] == netnsPath {
			hasOurNetns = true
		}
	}

	return isPasta && hasOurNetns
}

// findPastaProcess finds the PID of the pasta process for the given netns path.
// It searches /proc for processes matching the pasta command with the specified netns.
//
// Security note: This function matches processes by cmdline, which could theoretically
// match a different process if the netns path is reused. However, this is low risk because:
// 1. The netns path includes the container's network namespace which is unique while active
// 2. A false positive would only result in terminating a process that's using our netns
// 3. The worst case is an ESRCH error if the process exits between detection and termination
func findPastaProcess(netnsPath string) (int, error) {
	// Read /proc to find pasta processes
	procDir, err := os.Open("/proc")
	if err != nil {
		return 0, err
	}
	defer procDir.Close()

	// -1 reads all directory entries at once
	entries, err := procDir.Readdirnames(procReadBatchSize)
	if err != nil {
		return 0, err
	}

	for _, entry := range entries {
		// Skip non-numeric entries (only look at PIDs)
		pid, err := strconv.Atoi(entry)
		if err != nil {
			continue
		}

		// Read the command line
		cmdlineBytes, err := os.ReadFile(filepath.Join("/proc", entry, "cmdline"))
		if err != nil {
			continue
		}

		cmdline := string(cmdlineBytes)
		// cmdline has null-separated arguments
		args := strings.Split(cmdline, "\x00")

		if matchPastaCmdline(args, netnsPath) {
			return pid, nil
		}
	}

	return 0, nil
}
