package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/containers/podman/v5/libpod/define"
)

func TestFormatError(t *testing.T) {
	err := errors.New("unknown error")
	output := formatError(err)
	expected := fmt.Sprintf("Error: %v", err)

	if output != expected {
		t.Errorf("Expected \"%s\" to equal \"%s\"", output, err.Error())
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
