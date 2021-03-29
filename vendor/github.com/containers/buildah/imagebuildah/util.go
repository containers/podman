package imagebuildah

import (
	"github.com/containers/buildah"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// InitReexec is a wrapper for buildah.InitReexec().  It should be called at
// the start of main(), and if it returns true, main() should return
// immediately.
func InitReexec() bool {
	return buildah.InitReexec()
}

func convertMounts(mounts []Mount) []specs.Mount {
	specmounts := []specs.Mount{}
	for _, m := range mounts {
		s := specs.Mount{
			Destination: m.Destination,
			Type:        m.Type,
			Source:      m.Source,
			Options:     m.Options,
		}
		specmounts = append(specmounts, s)
	}
	return specmounts
}
