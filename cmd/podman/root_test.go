package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"go.podman.io/podman/v6/libpod/define"
)

func TestFormatError(t *testing.T) {
	err := errors.New("unknown error")
	output := formatError(err)
	expected := fmt.Sprintf("Error: %v", err)

	if output != expected {
		t.Errorf("Expected \"%s\" to equal \"%s\"", output, err.Error())
	}
}

func TestIndentExamples(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "flush-left lines get indented",
			input:    "podman top ctrID\npodman top ctrID pid seccomp",
			expected: "  podman top ctrID\n  podman top ctrID pid seccomp",
		},
		{
			name:     "preserves empty lines between examples",
			input:    "podman run alpine\n\npodman run busybox",
			expected: "  podman run alpine\n\n  podman run busybox",
		},
		{
			name:     "handles comment lines",
			input:    "# List connections\npodman system connection ls",
			expected: "  # List connections\n  podman system connection ls",
		},
		{
			name:     "single line",
			input:    "podman run alpine",
			expected: "  podman run alpine",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := indentExamples(tt.input); got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestFormatOCIError(t *testing.T) {
	expectedPrefix := "Error: "
	expectedSuffix := "OCI runtime output"
	err := fmt.Errorf("%s: %w", expectedSuffix, define.ErrOCIRuntime)
	output := formatError(err)

	if !strings.HasPrefix(output, expectedPrefix) {
		t.Errorf("Expected \"%s\" to start with \"%s\"", output, expectedPrefix)
	}
	if !strings.HasSuffix(output, expectedSuffix) {
		t.Errorf("Expected \"%s\" to end with \"%s\"", output, expectedSuffix)
	}
}
