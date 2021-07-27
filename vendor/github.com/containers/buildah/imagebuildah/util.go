package imagebuildah

import (
	"github.com/containers/buildah"
)

// InitReexec is a wrapper for buildah.InitReexec().  It should be called at
// the start of main(), and if it returns true, main() should return
// successfully immediately.
func InitReexec() bool {
	return buildah.InitReexec()
}
