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
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// procReadBatchSize is the number of /proc entries to read at once
	// -1 means read all entries at once, which is acceptable for /proc
	procReadBatchSize = -1
)

// matchPastaCmdline checks if the given command line arguments represent a pasta
// process using the specified network namespace path.
// Returns true if the args contain "pasta" in any argument (to match both "pasta"
// and "/usr/bin/pasta") AND have "--netns" followed by the matching netnsPath.
func matchPastaCmdline(args []string, netnsPath string) bool {
	if len(args) == 0 || netnsPath == "" {
		return false
	}

	isPasta := false
	hasOurNetns := false

	for i, arg := range args {
		if strings.Contains(arg, "pasta") {
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
