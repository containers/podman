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

// argsMapToSlice returns the contents of a map[string]string as a slice of keys
// and values joined with "=".
func argsMapToSlice(m map[string]string) []string {
	s := make([]string, 0, len(m))
	for k, v := range m {
		s = append(s, k+"="+v)
	}
	return s
}
