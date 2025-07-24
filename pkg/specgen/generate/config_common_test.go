//go:build !remote

package generate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseDevice(t *testing.T) {
	tests := []struct {
		device string
		src    string
		dst    string
		perm   string
	}{
		{"/dev/foo", "/dev/foo", "/dev/foo", "rwm"},
		{"/dev/foo:/dev/bar", "/dev/foo", "/dev/bar", "rwm"},
		{"/dev/foo:/dev/bar:rw", "/dev/foo", "/dev/bar", "rw"},
		{"/dev/foo:rw", "/dev/foo", "/dev/foo", "rw"},
		{"/dev/foo::rw", "/dev/foo", "/dev/foo", "rw"},
	}
	for _, test := range tests {
		src, dst, perm, err := ParseDevice(test.device)
		assert.NoError(t, err)
		assert.Equal(t, src, test.src)
		assert.Equal(t, dst, test.dst)
		assert.Equal(t, perm, test.perm)
	}
}

func TestParseDeviceErrors(t *testing.T) {
	errorTests := []struct {
		device        string
		expectedError string
	}{
		{"/dev/fuse::", "empty device mode in device specification: /dev/fuse::"},
		{"/dev/fuse:::rw", "invalid device specification: /dev/fuse:::rw"},
		{"/dev/fuse:invalid", "invalid device mode: invalid in device specification: /dev/fuse:invalid"},
		{"/dev/fuse:/path:xyz", "invalid device mode: xyz in device specification: /dev/fuse:/path:xyz"},
	}

	for _, test := range errorTests {
		_, _, _, err := ParseDevice(test.device)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), test.expectedError)
	}
}
