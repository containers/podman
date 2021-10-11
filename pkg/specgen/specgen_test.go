package specgen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSpecGeneratorWithRootfs(t *testing.T) {
	tests := []struct {
		rootfs                string
		expectedRootfsOverlay bool
		expectedRootfs        string
	}{
		{"/root/a:b:O", true, "/root/a:b"},
		{"/root/a:b/c:O", true, "/root/a:b/c"},
		{"/root/a:b/c:", false, "/root/a:b/c:"},
		{"/root/a/b", false, "/root/a/b"},
	}
	for _, args := range tests {
		val := NewSpecGenerator(args.rootfs, true)
		assert.Equal(t, val.RootfsOverlay, args.expectedRootfsOverlay)
		assert.Equal(t, val.Rootfs, args.expectedRootfs)
	}
}
