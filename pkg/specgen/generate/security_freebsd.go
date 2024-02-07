//go:build !remote

package generate

import (
	"github.com/containers/common/libimage"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/opencontainers/runtime-tools/generate"
)

// setLabelOpts sets the label options of the SecurityConfig according to the
// input.
func setLabelOpts(s *specgen.SpecGenerator, runtime *libpod.Runtime, pidConfig specgen.Namespace, ipcConfig specgen.Namespace) error {
	return nil
}

func securityConfigureGenerator(s *specgen.SpecGenerator, g *generate.Generator, newImage *libimage.Image, rtc *config.Config) error {
	// If this is a privileged container, change the devfs ruleset to expose all devices.
	if s.IsPrivileged() {
		for k, m := range g.Config.Mounts {
			if m.Type == "devfs" {
				m.Options = []string{
					"ruleset=0",
				}
				g.Config.Mounts[k] = m
			}
		}
	}

	if s.ReadOnlyFilesystem != nil {
		g.SetRootReadonly(*s.ReadOnlyFilesystem)
	}

	return nil
}
