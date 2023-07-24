package generate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldMask(t *testing.T) {
	tests := []struct {
		mask       string
		unmask     []string
		shouldMask bool
	}{
		{"/proc/foo", []string{"all"}, false},
		{"/proc/foo", []string{"ALL"}, false},
		{"/proc/foo", []string{"/proc/foo"}, false},
		{"/proc/foo", []string{"/proc/*"}, false},
		{"/proc/foo", []string{"/proc/bar", "all"}, false},
		{"/proc/foo", []string{"/proc/f*"}, false},
		{"/proc/foo", []string{"/proc/b*"}, true},
		{"/proc/foo", []string{}, true},
	}
	for _, test := range tests {
		val := shouldMask(test.mask, test.unmask)
		assert.Equal(t, val, test.shouldMask)
	}
}

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
