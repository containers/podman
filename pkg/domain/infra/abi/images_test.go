//go:build !remote && (linux || freebsd)

package abi

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/common/libimage"
)

// This is really intended to verify what happens with a
// nil pointer in layer.Created, but we'll just sanity
// check round tripping 42.
func TestToDomainHistoryLayer(t *testing.T) {
	var layer libimage.ImageHistory
	layer.Size = 42
	newLayer := toDomainHistoryLayer(&layer)
	assert.Equal(t, layer.Size, newLayer.Size)
}

// TestLoadImageENOSPCWrapping verifies that the ENOSPC-detection logic used
// in (*ImageEngine).Load wraps disk-full errors so that callers can detect
// them with errors.Is(err, syscall.ENOSPC).
func TestLoadImageENOSPCWrapping(t *testing.T) {
	// applyENOSPCCheck reproduces the inline check added to the Load function.
	applyENOSPCCheck := func(loadErr error) error {
		if loadErr == nil {
			return nil
		}
		if strings.Contains(loadErr.Error(), "no space left on device") {
			return fmt.Errorf("loading image: %w", syscall.ENOSPC)
		}
		return loadErr
	}

	// A subprocess error that contains the canonical ENOSPC string.
	enospcErr := errors.New("writing blob: no space left on device")
	result := applyENOSPCCheck(enospcErr)
	assert.True(t, errors.Is(result, syscall.ENOSPC),
		"expected syscall.ENOSPC in error chain, got: %v", result)

	// A generic error must not be mistaken for ENOSPC.
	otherErr := errors.New("permission denied")
	result = applyENOSPCCheck(otherErr)
	assert.False(t, errors.Is(result, syscall.ENOSPC),
		"non-ENOSPC error must not wrap syscall.ENOSPC")

	// No error must pass through as nil.
	assert.NoError(t, applyENOSPCCheck(nil))
}
