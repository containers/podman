package specgen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSpecGeneratorWithRootfs(t *testing.T) {
	idmap := "idmap"
	idmapMappings := "idmap=uids=1-1-2000"
	tests := []struct {
		rootfs                string
		expectedRootfsOverlay bool
		expectedRootfs        string
		expectedMapping       *string
	}{
		{"/root/a:b:O", true, "/root/a:b", nil},
		{"/root/a:b/c:O", true, "/root/a:b/c", nil},
		{"/root/a:b/c:", false, "/root/a:b/c:", nil},
		{"/root/a/b", false, "/root/a/b", nil},
		{"/root/a:b/c:idmap", false, "/root/a:b/c", &idmap},
		{"/root/a:b/c:idmap=uids=1-1-2000", false, "/root/a:b/c", &idmapMappings},
	}
	for _, args := range tests {
		val := NewSpecGenerator(args.rootfs, true)

		assert.Equal(t, val.RootfsOverlay, args.expectedRootfsOverlay)
		assert.Equal(t, val.Rootfs, args.expectedRootfs)
		if args.expectedMapping == nil {
			assert.Nil(t, val.RootfsMapping)
		} else {
			assert.NotNil(t, val.RootfsMapping)
			assert.Equal(t, *val.RootfsMapping, *args.expectedMapping)
		}
	}
}
