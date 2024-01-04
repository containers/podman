//go:build !remote

package libpod

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLabelVolumePath(t *testing.T) {
	// Set up mocked SELinux functions for testing.
	oldRelabel := lvpRelabel
	oldInitLabels := lvpInitLabels
	oldReleaseLabel := lvpReleaseLabel
	defer func() {
		lvpRelabel = oldRelabel
		lvpInitLabels = oldInitLabels
		lvpReleaseLabel = oldReleaseLabel
	}()

	// Relabel returns ENOTSUP unconditionally.
	lvpRelabel = func(path string, fileLabel string, shared bool) error {
		return syscall.ENOTSUP
	}

	// InitLabels and ReleaseLabel both return dummy values and nil errors.
	lvpInitLabels = func(options []string) (string, string, error) {
		pLabel := "system_u:system_r:container_t:s0:c1,c2"
		mLabel := "system_u:object_r:container_file_t:s0:c1,c2"
		return pLabel, mLabel, nil
	}
	lvpReleaseLabel = func(label string) error {
		return nil
	}

	// LabelVolumePath should not return an error if the operation is unsupported.
	err := LabelVolumePath("/foo/bar", "")
	assert.NoError(t, err)
}
