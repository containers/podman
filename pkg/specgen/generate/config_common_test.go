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
