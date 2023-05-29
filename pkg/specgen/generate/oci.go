package generate

import (
	"fmt"
	"strings"

	"github.com/containers/common/libimage"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/opencontainers/runtime-tools/generate"
)

func addRlimits(s *specgen.SpecGenerator, g *generate.Generator) {
	g.Config.Process.Rlimits = nil

	for _, u := range s.Rlimits {
		name := "RLIMIT_" + strings.ToUpper(u.Type)
		g.AddProcessRlimits(name, u.Hard, u.Soft)
	}
}

// Produce the final command for the container.
func makeCommand(s *specgen.SpecGenerator, imageData *libimage.ImageData, rtc *config.Config) ([]string, error) {
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
		return nil, fmt.Errorf("no command or entrypoint provided, and no CMD or ENTRYPOINT from image")
	}

	if s.Init {
		initPath := s.InitPath
		if initPath == "" && rtc != nil {
			initPath = rtc.Engine.InitPath
		}
		if initPath == "" {
			return nil, fmt.Errorf("no path to init binary found but container requested an init")
		}
		finalCommand = append([]string{define.ContainerInitPath, "--"}, finalCommand...)
	}

	return finalCommand, nil
}
