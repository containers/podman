//go:build !remote

package generate

import (
	"strings"

	"github.com/containers/common/libimage"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/sirupsen/logrus"
)

func addRlimits(s *specgen.SpecGenerator, g *generate.Generator) {
	g.Config.Process.Rlimits = nil

	for _, u := range s.Rlimits {
		name := "RLIMIT_" + strings.ToUpper(u.Type)
		u = subNegativeOne(u)
		g.AddProcessRlimits(name, u.Hard, u.Soft)
	}
}

// Produce the final command for the container.
func makeCommand(s *specgen.SpecGenerator, imageData *libimage.ImageData) []string {
	finalCommand := []string{}

	entrypoint := s.Entrypoint
	if entrypoint == nil && imageData != nil {
		entrypoint = imageData.Config.Entrypoint
	}

	// Don't append the entrypoint if it is [""]
	if len(entrypoint) != 1 || entrypoint[0] != "" {
		finalCommand = append(finalCommand, entrypoint...)
	}

	// Only use image command if the user did not manually set an
	// entrypoint.
	command := s.Command
	if len(command) == 0 && imageData != nil && len(s.Entrypoint) == 0 {
		command = imageData.Config.Cmd
	}

	finalCommand = append(finalCommand, command...)

	if len(finalCommand) == 0 {
		logrus.Debug("no command or entrypoint provided, and no CMD or ENTRYPOINT from image: defaulting to empty string")
		finalCommand = []string{""}
	}

	if s.Init != nil && *s.Init {
		// bind mount for this binary is added in addContainerInitBinary()
		finalCommand = append([]string{define.ContainerInitPath, "--"}, finalCommand...)
	}

	return finalCommand
}
